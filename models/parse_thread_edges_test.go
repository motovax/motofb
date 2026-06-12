package models

import "testing"

func TestParseThreadsParticipantEdges(t *testing.T) {
	raw := map[string]any{
		"thread_key":  map[string]any{"thread_fbid": "914567754270085"},
		"thread_type": "MARKETPLACE",
		"folder":      "INBOX",
		"all_participants": map[string]any{
			"edges": []any{
				map[string]any{"node": map[string]any{"messaging_actor": map[string]any{"id": "1514743436", "name": "Self"}}},
				map[string]any{"node": map[string]any{"messaging_actor": map[string]any{"id": "100024624619655", "name": "Other"}}},
			},
		},
	}
	threads, err := ParseThreads([]any{raw})
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 {
		t.Fatalf("threads=%d", len(threads))
	}
	if len(threads[0].AllParticipants) != 2 {
		t.Fatalf("participants=%d", len(threads[0].AllParticipants))
	}
}