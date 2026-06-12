package state_test

import (
	"context"
	"os"
	"testing"

	"github.com/motovax/motofb/graphql"
	"github.com/motovax/motofb/state"
)

func TestGraphQLBatchHeadersIncludeLSDAndSecFetch(t *testing.T) {
	st := &state.State{
		LSD:       "lsd-token",
		UserAgent: "Mozilla/5.0",
	}
	h := st.BuildHeaders("https://www.facebook.com/api/graphqlbatch/", "post", "")
	if h.Get("X-Fb-Lsd") != "lsd-token" {
		t.Fatalf("X-Fb-Lsd=%q", h.Get("X-Fb-Lsd"))
	}
	if h.Get("Sec-Fetch-Mode") != "cors" {
		t.Fatalf("Sec-Fetch-Mode=%q", h.Get("Sec-Fetch-Mode"))
	}
}

func TestGraphQLBatchNamedIntegration(t *testing.T) {
	data, err := os.ReadFile("/tmp/fb_cookies.json")
	if err != nil {
		t.Skip("no /tmp/fb_cookies.json")
	}
	jar, err := state.LoadCookiesFromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Login(context.Background(), jar, state.Options{})
	if err != nil {
		t.Fatal(err)
	}
	q := graphql.FromDocID(graphql.DocThreadList, map[string]any{
		"limit": 3, "tags": "INBOX", "before": nil,
		"includeDeliveryReceipts": true, "includeSeqID": false,
	})
	result, err := st.GraphQLBatchNamed(context.Background(), "MessengerGraphQLThreadlistFetcher", q)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Fatal("empty result")
	}
	viewer, _ := result[0]["viewer"].(map[string]any)
	if viewer == nil {
		t.Fatal("missing viewer")
	}
}