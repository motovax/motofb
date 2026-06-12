package models

import (
	"fmt"
	"strings"
)

// Mentions groups @user references for send_message (fbchat_muqit.Mentions parity).
type Mentions struct {
	Users []Mention
}

// MentionSpec pairs a user id with the display name to locate in message text.
type MentionSpec struct {
	UserID string
	Name   string
}

// MentionsFromText builds mentions by locating each name in text, matching
// fbchat_muqit.Mentions.from_text(text, [(user_id, name), ...]).
func MentionsFromText(text string, users []MentionSpec) (Mentions, error) {
	if len(users) == 0 {
		return Mentions{}, nil
	}
	out := make([]Mention, 0, len(users))
	for _, u := range users {
		name := strings.TrimSpace(u.Name)
		if u.UserID == "" {
			return Mentions{}, fmt.Errorf("mention user id is required")
		}
		if name == "" {
			return Mentions{}, fmt.Errorf("mention name is required for user %s", u.UserID)
		}
		offset := strings.Index(text, name)
		if offset < 0 {
			return Mentions{}, fmt.Errorf("name %q not found in text", name)
		}
		out = append(out, Mention{
			UserID: u.UserID,
			Name:   name,
			Offset: offset,
			Length: len(name),
		})
	}
	return Mentions{Users: out}, nil
}

// Payload returns the structure used by SendMessage mention_data.
func (m Mentions) Payload() MentionsPayload {
	return MentionsPayload{Users: m.Users}
}