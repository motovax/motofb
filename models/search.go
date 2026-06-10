package models

import "encoding/json"

// MessageSearchResult is one matched message from search.
type MessageSearchResult struct {
	Query            string
	ResultIndex      int
	ThreadID         string
	SenderName       string
	MessageID        string
	TimestampMs      int64
	Snippet          string
	ProfilePicURL    string
	HighlightOffset  string
	HighlightLength  string
}

// MessageSearchStatus is search query metadata.
type MessageSearchStatus struct {
	Query        string
	TotalResults int
	Status       int
	HasMore      bool
	NextCursor   string
	ThreadID     string
}

// MessageSearchResponse combines search hits and status.
type MessageSearchResponse struct {
	Results []MessageSearchResult
	Status  *MessageSearchStatus
}

// ParseMessageSearch walks Facebook's encoded ls_resp search payload.
func ParseMessageSearch(data string) MessageSearchResponse {
	var root any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return MessageSearchResponse{}
	}
	out := MessageSearchResponse{}
	walkSearchNode(root, &out)
	return out
}

func walkSearchNode(node any, out *MessageSearchResponse) {
	switch n := node.(type) {
	case []any:
		if len(n) >= 2 && intVal(n[0]) == 5 {
			if op, ok := n[1].(string); ok {
				args := make([]any, 0, len(n)-2)
				for _, v := range n[2:] {
					args = append(args, decodeSearchValue(v))
				}
				switch op {
				case "insertMessageSearchResult":
					if len(args) >= 13 {
						out.Results = append(out.Results, MessageSearchResult{
							Query:           strVal(args[0]),
							ResultIndex:       intVal(args[1]),
							ThreadID:          strVal(args[2]),
							SenderName:        strVal(args[5]),
							MessageID:         strVal(args[6]),
							TimestampMs:       int64(intVal(args[7])),
							Snippet:           strVal(args[8]),
							ProfilePicURL:     strVal(args[9]),
							HighlightOffset:   strVal(args[11]),
							HighlightLength:   strVal(args[12]),
						})
					}
				case "updateMessageSearchQueryStatus":
					if len(args) >= 6 {
						out.Status = &MessageSearchStatus{
							Query:        strVal(args[0]),
							TotalResults: intVal(args[1]),
							Status:       intVal(args[2]),
							HasMore:      toBool(args[3]),
							NextCursor:   strVal(args[4]),
							ThreadID:     strVal(args[5]),
						}
					}
				}
			}
		}
		for _, item := range n {
			walkSearchNode(item, out)
		}
	case map[string]any:
		for _, v := range n {
			walkSearchNode(v, out)
		}
	}
}

func decodeSearchValue(v any) any {
	arr, ok := v.([]any)
	if !ok || len(arr) < 2 {
		return v
	}
	switch intVal(arr[0]) {
	case 19:
		if s, ok := arr[1].(string); ok {
			return s
		}
	case 9:
		return nil
	}
	return v
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	default:
		return false
	}
}

