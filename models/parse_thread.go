package models

import (
	"encoding/json"
	"fmt"
)

var threadTypeMap = map[string]ThreadType{
	"GROUP":      ThreadTypeGroup,
	"ONE_TO_ONE": ThreadTypeUser,
	"PAGE":       ThreadTypePage,
	"COMMUNITY":  ThreadTypeCommunity,
}

// ParseThreads converts GraphQL thread list nodes to Thread values.
func ParseThreads(data any) ([]Thread, error) {
	nodes, err := extractThreadNodes(data)
	if err != nil {
		return nil, err
	}
	out := make([]Thread, 0, len(nodes))
	for _, n := range nodes {
		t, err := parseThreadNode(n)
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func extractThreadNodes(data any) ([]map[string]any, error) {
	switch v := data.(type) {
	case []map[string]any:
		return v, nil
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	case map[string]any:
		if mt, ok := v["message_thread"].(map[string]any); ok {
			return []map[string]any{mt}, nil
		}
		if viewer, ok := v["viewer"].(map[string]any); ok {
			if mt, ok := viewer["message_threads"].(map[string]any); ok {
				if nodes, ok := mt["nodes"].([]any); ok {
					return mapsFromSlice(nodes), nil
				}
			}
		}
		return []map[string]any{v}, nil
	default:
		return nil, fmt.Errorf("unsupported thread data type")
	}
}

func parseThreadNode(t map[string]any) (Thread, error) {
	if mt, ok := t["message_thread"].(map[string]any); ok {
		t = mt
	}
	threadKey, _ := t["thread_key"].(map[string]any)
	threadID := strVal(threadKey["thread_fbid"])
	if threadID == "" {
		threadID = strVal(threadKey["other_user_id"])
	}
	tt := ThreadTypeUnknown
	if s, ok := t["thread_type"].(string); ok {
		tt = threadTypeMap[s]
	}
	folder := ThreadFolderInbox
	if s, ok := t["folder"].(string); ok {
		folder = ThreadFolder(s)
	}
	var image string
	if img, ok := t["image"].(map[string]any); ok {
		image = strVal(img["uri"])
	}
	var joinableLink string
	var joinableMode int
	if jm, ok := t["joinable_mode"].(map[string]any); ok {
		joinableLink = strVal(jm["link"])
		joinableMode = intVal(jm["mode"])
	}
	admins := []string{}
	if arr, ok := t["thread_admins"].([]any); ok {
		for _, a := range arr {
			if m, ok := a.(map[string]any); ok {
				admins = append(admins, strVal(m["id"]))
			}
		}
	}
	return Thread{
		Name:            strVal(t["name"]),
		ThreadID:        threadID,
		MessageCount:    intVal(t["messages_count"]),
		Image:           image,
		ThreadType:      tt,
		Folder:          folder,
		AllParticipants: parseThreadParticipants(t),
		ThreadAdmins:    admins,
		ApprovalMode:    intVal(t["approval_mode"]),
		JoinableMode:    joinableMode,
		JoinableLink:    joinableLink,
		PrivacyMode:     intVal(t["privacy_mode"]),
		IsJoined:        boolVal(t["is_viewer_subscribed"]),
		IsPinned:        boolVal(t["is_pinned"]),
		Description:     strVal(t["description"]),
	}, nil
}

func parseThreadParticipants(t map[string]any) []User {
	raw, ok := t["all_participants"].(map[string]any)
	if !ok {
		return nil
	}
	nodes, ok := raw["nodes"].([]any)
	if !ok {
		return nil
	}
	out := make([]User, 0, len(nodes))
	for _, item := range nodes {
		node, ok := item.(map[string]any)
		if !ok {
			continue
		}
		messaging, _ := node["messaging_actor"].(map[string]any)
		if messaging == nil {
			messaging = node
		}
		id := strVal(messaging["id"])
		if id == "" {
			id = strVal(node["id"])
		}
		if id == "" {
			continue
		}
		out = append(out, User{
			ID:        id,
			Name:      strVal(messaging["name"]),
			FirstName: strVal(messaging["short_name"]),
			Image:     strVal(messaging["big_image_src"]),
		})
	}
	return out
}

// ParseUsersFromGraphQL extracts users from /chat/user_info responses.
func ParseUsersFromGraphQL(payload map[string]any) map[string]User {
	out := map[string]User{}
	p, _ := payload["payload"].(map[string]any)
	if p == nil {
		return out
	}
	profiles, _ := p["profiles"].(map[string]any)
	for k, raw := range profiles {
		v, _ := raw.(map[string]any)
		out[k] = User{
			ID:        strVal(v["id"]),
			Name:      strVal(v["name"]),
			FirstName: strVal(v["firstName"]),
			Username:  strVal(v["vanity"]),
			Gender:    strVal(v["gender"]),
			URL:       strVal(v["uri"]),
			IsFriend:  boolVal(v["is_viewer_friend"]),
			IsBlocked: boolVal(v["is_message_blocked_by_viewer"]),
			Image:     strVal(v["big_image_src"]),
		}
	}
	return out
}

func mapsFromSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func strVal(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return fmt.Sprintf("%.0f", x)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func intVal(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	default:
		return 0
	}
}

func boolVal(v any) bool {
	b, _ := v.(bool)
	return b
}