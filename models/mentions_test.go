package models

import "testing"

func TestMentionsFromText(t *testing.T) {
	text := "Hey Alice and Bob!"
	mentions, err := MentionsFromText(text, []MentionSpec{
		{UserID: "100001111111111", Name: "Alice"},
		{UserID: "100002222222222", Name: "Bob"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mentions.Users) != 2 {
		t.Fatalf("expected 2 mentions, got %d", len(mentions.Users))
	}
	if mentions.Users[0].Offset != 4 || mentions.Users[0].Length != 5 {
		t.Fatalf("alice mention: got offset=%d length=%d", mentions.Users[0].Offset, mentions.Users[0].Length)
	}
	if mentions.Users[1].Offset != 14 || mentions.Users[1].Length != 3 {
		t.Fatalf("bob mention: got offset=%d length=%d", mentions.Users[1].Offset, mentions.Users[1].Length)
	}
	payload := mentions.Payload().ToMap()
	if payload == nil {
		t.Fatal("expected mention payload")
	}
	if payload["mention_ids"] != "100001111111111,100002222222222" {
		t.Fatalf("mention_ids: %v", payload["mention_ids"])
	}
}

func TestMentionsFromTextMissingName(t *testing.T) {
	_, err := MentionsFromText("hello", []MentionSpec{{UserID: "1", Name: "Missing"}})
	if err == nil {
		t.Fatal("expected error when name is absent from text")
	}
}