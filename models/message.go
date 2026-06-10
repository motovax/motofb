package models

// Mention is an @user reference in message text.
type Mention struct {
	UserID string `json:"i"`
	Offset int    `json:"o"`
	Length int    `json:"l"`
	Name   string `json:"name,omitempty"`
}

// MentionsPayload builds mention_data for send_message.
type MentionsPayload struct {
	Users []Mention
}

func (m MentionsPayload) ToMap() map[string]any {
	if len(m.Users) == 0 {
		return nil
	}
	ids, offsets, lengths, types := make([]string, 0, len(m.Users)), make([]string, 0, len(m.Users)), make([]string, 0, len(m.Users)), make([]string, 0, len(m.Users))
	for _, u := range m.Users {
		ids = append(ids, u.UserID)
		offsets = append(offsets, itoa(u.Offset))
		lengths = append(lengths, itoa(u.Length))
		types = append(types, "p")
	}
	return map[string]any{
		"mention_ids":     join(ids),
		"mention_offsets": join(offsets),
		"mention_lengths": join(lengths),
		"mention_types":   join(types),
	}
}

// Message is an incoming or fetched Messenger message.
type Message struct {
	ID                string
	Text              string
	SenderID          string
	ThreadID          string
	ThreadType        ThreadType
	ThreadFolder      ThreadFolder
	MessageType       MessageType
	Mentions          []Mention
	Attachments       []Attachment
	Timestamp         int64
	CanUnsend         bool
	Unsent            bool
	RepliedToMessage  *Message
	ThreadParticipants []int64
	Reactions         []MessageReaction
}

// MessageReaction is a reaction event.
type MessageReaction struct {
	ID                   string
	ThreadID             string
	Reactor              int64
	ReactedMessageSender int64
	ReactionType         ReactionAction
	Reaction             string
	Timestamp            *int64
}

// MessageUnsend is an unsent message event.
type MessageUnsend struct {
	ID        string
	ThreadID  string
	SenderID  string
	Timestamp int64
}

// MessageRemove is client-side message removal.
type MessageRemove struct {
	IDs      []string
	ThreadID string
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func join(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += "," + parts[i]
	}
	return out
}