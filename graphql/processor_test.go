package graphql_test

import (
	"testing"

	"github.com/motovax/motofb/graphql"
)

func TestStripJSONCruft(t *testing.T) {
	p := graphql.NewProcessor()
	out, err := p.StripJSONCruft(`for(;;);{"ok":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"ok":true}` {
		t.Fatalf("got %q", out)
	}
}

func TestParseJSONStream(t *testing.T) {
	p := graphql.NewProcessor()
	objs, err := p.ParseJSONStream(`{"a":1}{"b":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("len=%d", len(objs))
	}
}

func TestQueriesToJSON(t *testing.T) {
	p := graphql.NewProcessor()
	s := p.QueriesToJSON(graphql.FromDocID("123", map[string]any{"limit": 5}))
	if s == "" || s[0] != '{' {
		t.Fatalf("unexpected json: %q", s)
	}
}

func TestProcessResponse(t *testing.T) {
	p := graphql.NewProcessor()
	raw := `{"q0":{"data":{"viewer":{"id":"1"}}}}`
	results, err := p.ProcessResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0] == nil {
		t.Fatalf("results=%v", results)
	}
}