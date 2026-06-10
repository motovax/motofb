package graphql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	fberr "github.com/motovax/motofb/errors"
)

// Facebook API error codes from upstream graphql.py.
const (
	ErrNotLoggedIn    = 1357001
	ErrRefreshCookies = 1357004
	ErrInvalidParams1 = 1357031
	ErrInvalidParams2 = 1545010
	ErrInvalidParams3 = 1545003
)

// Processor handles GraphQL query encoding and response parsing.
type Processor struct{}

func NewProcessor() *Processor { return &Processor{} }

// StripJSONCruft removes Facebook's for(;;); prefix.
func (p *Processor) StripJSONCruft(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fberr.Wrap("StripJSONCruft", "empty content", fberr.ErrValidation)
	}
	idx := strings.Index(content, "{")
	if idx < 0 {
		return "", fberr.Wrap("StripJSONCruft", "no json object found", fberr.ErrValidation)
	}
	return content[idx:], nil
}

// ParseJSONStream decodes concatenated JSON objects.
func (p *Processor) ParseJSONStream(content string) ([]map[string]any, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	var results []map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(content)))
	for dec.More() {
		var obj map[string]any
		if err := dec.Decode(&obj); err != nil {
			return results, fberr.Wrap("ParseJSONStream", "decode json", err)
		}
		results = append(results, obj)
	}
	return results, nil
}

// ProcessNormalResponse parses a single JSON object response.
func (p *Processor) ProcessNormalResponse(content string) (map[string]any, error) {
	cleaned, err := p.StripJSONCruft(content)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return nil, fberr.Wrap("ProcessNormalResponse", "unmarshal", err)
	}
	return out, nil
}

// QueriesToJSON serializes QueryRequest values for graphqlbatch.
func (p *Processor) QueriesToJSON(queries ...QueryRequest) string {
	rtn := make(map[string]any, len(queries))
	for i, q := range queries {
		key := fmt.Sprintf("q%d", i)
		switch {
		case q.Query != "":
			rtn[key] = map[string]any{"priority": q.Priority, "q": q.Query, "query_params": q.QueryParams}
		case q.QueryID != "":
			rtn[key] = map[string]any{"query_id": q.QueryID, "query_params": q.QueryParams}
		case q.Doc != "":
			rtn[key] = map[string]any{"doc": q.Doc, "query_params": q.QueryParams}
		case q.DocID != "":
			rtn[key] = map[string]any{"doc_id": q.DocID, "query_params": q.QueryParams}
		}
	}
	b, _ := json.Marshal(rtn)
	return string(b)
}

// HandlePayloadError checks top-level Facebook error codes.
func (p *Processor) HandlePayloadError(payload map[string]any) error {
	raw, ok := payload["error"]
	if !ok {
		return nil
	}
	code, _ := toInt(raw)
	switch code {
	case ErrNotLoggedIn:
		return fberr.WithCode("HandlePayloadError", "not logged in", code)
	case ErrRefreshCookies:
		return fberr.WithCode("HandlePayloadError", "refresh cookies", code)
	case ErrInvalidParams1, ErrInvalidParams2, ErrInvalidParams3:
		return fberr.WithCode("HandlePayloadError", "invalid parameters", code)
	default:
		return fberr.WithCode("HandlePayloadError", fmt.Sprintf("facebook error %v", raw), code)
	}
}

// ProcessResponse parses graphqlbatch multi-query responses.
func (p *Processor) ProcessResponse(content string) ([]map[string]any, error) {
	cleaned, err := p.StripJSONCruft(content)
	if err != nil {
		return nil, err
	}
	objects, err := p.ParseJSONStream(cleaned)
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, obj := range objects {
		if _, skip := obj["error_results"]; skip {
			continue
		}
		if err := p.HandlePayloadError(obj); err != nil {
			return nil, err
		}
		for key, value := range obj {
			if len(key) < 2 || key[0] != 'q' {
				continue
			}
			idx, err := strconv.Atoi(key[1:])
			if err != nil {
				continue
			}
			for len(results) <= idx {
				results = append(results, nil)
			}
			vm, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if err := p.handleGraphQLErrors(vm); err != nil {
				return nil, err
			}
			if resp, ok := vm["response"].(map[string]any); ok {
				results[idx] = resp
			} else if data, ok := vm["data"].(map[string]any); ok {
				results[idx] = data
			} else {
				results[idx] = vm
			}
		}
	}
	return results, nil
}

func (p *Processor) handleGraphQLErrors(response map[string]any) error {
	var errorsRaw any
	if response["error"] != nil {
		errorsRaw = []any{response["error"]}
	} else if response["errors"] != nil {
		errorsRaw = response["errors"]
	} else {
		return nil
	}
	errs, ok := errorsRaw.([]any)
	if !ok || len(errs) == 0 {
		return nil
	}
	first, ok := errs[0].(map[string]any)
	if !ok {
		return nil
	}
	msg, _ := first["message"].(string)
	code, _ := toInt(first["code"])
	severity, _ := first["severity"].(string)
	if severity == "CRITICAL" || code != 0 {
		return fberr.WithCode("handleGraphQLErrors", msg, code)
	}
	return nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}