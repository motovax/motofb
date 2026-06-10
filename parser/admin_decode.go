package parser

import (
	"bytes"
	"encoding/json"

	"github.com/motovax/motofb/events"
	"github.com/motovax/motofb/models"
)

// AllAdminText matches Python group_deltas.AllAdminText.
var AllAdminText = map[string]struct{}{
	"joinable_group_link_mode_change":    {},
	"joinable_group_link_reset":          {},
	"change_thread_nickname":             {},
	"change_thread_theme":                {},
	"change_thread_admins":               {},
	"change_thread_quick_reaction":       {},
	"magic_words":                        {},
	"limit_sharing":                      {},
	"instant_game_dynamic_custom_update": {},
	"unpin_messages_v2":                  {},
	"pin_messages_v2":                    {},
}

var adminTextToEvent = map[string]events.Type{
	"joinable_group_link_reset":              events.ThreadJoinableLinkReset,
	"joinable_group_link_mode_change":        events.ThreadJoinableModeChange,
	"change_thread_nickname":                 events.NicknameChange,
	"change_thread_theme":                    events.ThemeChange,
	"change_thread_approval_mode":            events.ThreadApprovalMode,
	"change_thread_quick_reaction":           events.EmojiChange,
	"change_thread_admins":                   events.AdminAdded,
	"magic_words":                            events.ThreadMagicWords,
	"limit_sharing":                          events.ThreadMessageSharing,
	"instant_game_dynamic_custom_update":     events.ThreadGameChange,
	"pin_messages_v2":                        events.MessagePinned,
	"unpin_messages_v2":                      events.MessageUnpinned,
}

func decodeAdminUntyped(event events.Type, raw any) (any, error) {
	data, err := normalizeUntypedData(raw)
	if err != nil || len(data) == 0 {
		return raw, err
	}
	switch event {
	case events.AdminAdded:
		var out models.AdminAdded
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.MessagePinned:
		var out models.ThreadMessagePin
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.MessageUnpinned:
		var out models.ThreadMessageUnpin
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.ThreadMessageSharing:
		var out models.ThreadMessageSharing
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.ThreadMagicWords:
		var out models.ThreadMagicWord
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.NicknameChange:
		var out models.ThreadNickname
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.ThemeChange:
		var out models.ThreadTheme
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.EmojiChange:
		var out models.ThreadEmoji
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case events.ThreadJoinableLinkReset, events.ThreadJoinableModeChange:
		var out models.JoinableMode
		if err := json.Unmarshal(data, &out); err != nil {
			return map[string]any{}, nil
		}
		return out, nil
	case events.ThreadGameChange:
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return map[string]any{}, nil
		}
		return out, nil
	default:
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			return raw, err
		}
		return out, nil
	}
}

func normalizeUntypedData(raw any) ([]byte, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case []any:
		buf := make([]byte, 0, len(v))
		for _, item := range v {
			switch n := item.(type) {
			case float64:
				buf = append(buf, byte(int(n)))
			case int:
				buf = append(buf, byte(n))
			}
		}
		return buf, nil
	default:
		return json.Marshal(v)
	}
}

func containsRemoveAdmin(raw any) bool {
	data, err := normalizeUntypedData(raw)
	if err != nil || len(data) == 0 {
		return bytes.Contains([]byte(stringify(raw)), []byte("remove_admin"))
	}
	return bytes.Contains(data, []byte("remove_admin"))
}