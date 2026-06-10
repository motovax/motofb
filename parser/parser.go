package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/motovax/motofb/events"
	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/models"
)

// Parser decodes Messenger MQTT and GraphQL payloads using flexible JSON.
type Parser struct{}

// ParsedEvent is a decoded realtime event.
type ParsedEvent struct {
	EventType events.Type
	Args      []any
}

// New returns a message parser.
func New() *Parser { return &Parser{} }

var attachToMessage = map[models.AttachmentType]models.MessageType{
	models.AttachmentImage:           models.MessageTypeImage,
	models.AttachmentVideo:           models.MessageTypeVideo,
	models.AttachmentGIF:             models.MessageTypeGIF,
	models.AttachmentSticker:         models.MessageTypeSticker,
	models.AttachmentFile:            models.MessageTypeFile,
	models.AttachmentAudio:           models.MessageTypeAudio,
	models.AttachmentLocation:        models.MessageTypeLocation,
	models.AttachmentFacebookPost:    models.MessageTypeFacebookPost,
	models.AttachmentFacebookReel:    models.MessageTypeFacebookReel,
	models.AttachmentFacebookProfile: models.MessageTypeFacebookProfile,
	models.AttachmentFacebookGame:    models.MessageTypeFacebookPost,
	models.AttachmentFacebookProduct: models.MessageTypeFacebookProduct,
	models.AttachmentExternalURL:   models.MessageTypeExternalURL,
	models.AttachmentFacebookStory:   models.MessageTypeFacebookPost,
}

// ParseTMS parses /t_ms delta payloads.
func (p *Parser) ParseTMS(payload []byte) []ParsedEvent {
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil
	}
	deltas, _ := root["deltas"].([]any)
	out := make([]ParsedEvent, 0, len(deltas))
	for _, d := range deltas {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		if ev := p.ParseDeltas(m); ev != nil {
			out = append(out, *ev)
		}
	}
	return out
}

// ParseAll parses non-/t_ms MQTT topics.
func (p *Parser) ParseAll(topic string, payload []byte) *ParsedEvent {
	if (topic == "/thread_typing" && bytes.Contains(payload, []byte(`"type":"typ"`))) || topic == "/orca_typing_notifications" {
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			return nil
		}
		return &ParsedEvent{
			EventType: events.Typing,
			Args:      []any{parseTyping(raw)},
		}
	}
	if topic == "/orca_presence" && bytes.Contains(payload, []byte(`"list_type"`)) {
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			return nil
		}
		return &ParsedEvent{
			EventType: events.Presence,
			Args:      []any{parsePresence(raw)},
		}
	}
	if topic == "/legacy_web" && containsAny(payload, []byte("live_poke"), []byte("friending_state_change"), []byte("jewel_requests_remove_old"), []byte("mobile_requests_count")) {
		return p.parseNotifications(payload)
	}
	if bytes.Contains(payload, []byte(`"syncToken":"1"`)) {
		return nil
	}
	return nil
}

// ParseDeltas parses one delta object from /t_ms.
func (p *Parser) ParseDeltas(delta map[string]any) *ParsedEvent {
	class, _ := delta["class"].(string)
	switch class {
	case "NewMessage":
		msg := p.ParseMessage(delta, nil)
		return &ParsedEvent{EventType: events.Message, Args: []any{msg}}
	case "ClientPayload":
		return p.parseClientPayload(delta)
	case "ReadReceipt":
		return &ParsedEvent{EventType: events.MessageSeen, Args: []any{parseReadReceipt(delta)}}
	case "DeliveryReceipt":
		return &ParsedEvent{EventType: events.MessageDelivered, Args: []any{parseDeliveryReceipt(delta)}}
	case "MarkRead":
		return &ParsedEvent{EventType: events.MarkRead, Args: []any{parseMarkRead(delta)}}
	case "MarkUnread":
		return &ParsedEvent{EventType: events.MarkUnread, Args: []any{parseMarkUnread(delta)}}
	case "AdminRemoved":
		return &ParsedEvent{EventType: events.AdminRemoved, Args: []any{parseAdminRemoved(delta)}}
	case "ParticipantsAdded":
		return &ParsedEvent{EventType: events.ParticipantJoined, Args: []any{parseParticipantsAdded(delta)}}
	case "ParticipantLeft":
		return &ParsedEvent{EventType: events.ParticipantLeft, Args: []any{parseParticipantLeft(delta)}}
	case "ApprovalMode":
		return &ParsedEvent{EventType: events.ThreadApprovalMode, Args: []any{parseApprovalMode(delta)}}
	case "ApprovalQueue":
		return &ParsedEvent{EventType: events.ThreadApprovalQueue, Args: []any{parseApprovalQueue(delta)}}
	case "ThreadName":
		return &ParsedEvent{EventType: events.ThreadNameChange, Args: []any{parseThreadName(delta)}}
	case "ThreadAction":
		return &ParsedEvent{EventType: events.ThreadAction, Args: []any{parseThreadAction(delta)}}
	case "ThreadFolderMove":
		return &ParsedEvent{EventType: events.ThreadFolderMove, Args: []any{parseThreadFolderMove(delta)}}
	case "ThreadDelete":
		return &ParsedEvent{EventType: events.ThreadDelete, Args: []any{parseThreadDelete(delta)}}
	case "ThreadMuteSettings":
		return &ParsedEvent{EventType: events.ThreadMuteSettings, Args: []any{parseThreadMuteSettings(delta)}}
	case "AdminTextMessage":
		return p.parseAdminText(delta)
	default:
		return nil
	}
}

// ParseMessage builds a Message from delta or reply payload maps.
func (p *Parser) ParseMessage(data map[string]any, repliedTo *models.Message) models.Message {
	meta, _ := data["messageMetadata"].(map[string]any)
	if meta == nil {
		meta = data
	}
	attachments := p.parseAttachmentsFromDelta(data)
	msgType := models.MessageTypeText
	if len(attachments) > 0 {
		msgType = attachToMessage[attachments[0].Type]
	}
	folder := models.ThreadFolderInbox
	if f := unwrapStr(meta["folderId"]); f != "" {
		folder = models.ThreadFolder(f)
	}
	participants := intSlice(data["participants"])
	threadType := models.ThreadTypeUser
	if len(participants) >= 3 {
		threadType = models.ThreadTypeGroup
	}
	unsendType, _ := meta["unsendType"].(string)
	return models.Message{
		ID:                 strVal(meta["messageId"]),
		Text:               strVal(data["body"]),
		SenderID:           strVal(meta["actorFbId"]),
		MessageType:        msgType,
		Mentions:           p.parseMentionsFromDelta(data),
		ThreadID:           unwrapStr(meta["threadKey"]),
		ThreadType:         threadType,
		ThreadFolder:       folder,
		ThreadParticipants: participants,
		Attachments:        attachments,
		Timestamp:          int64(toInt(meta["timestamp"])),
		CanUnsend:          strings.EqualFold(unsendType, "Can_Unsend"),
		Unsent:             false,
		RepliedToMessage:   repliedTo,
	}
}

// ParseThreadMessage parses GraphQL thread message fetch results.
func (p *Parser) ParseThreadMessage(result any) ([]models.Message, error) {
	items, err := normalizeGraphQLBatch(result)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	root := items[0]
	mt, _ := root["message_thread"].(map[string]any)
	if mt == nil {
		return nil, fberr.Wrap("ParseThreadMessage", "missing message_thread", fberr.ErrParsing)
	}
	threadID := unwrapThreadKey(mt["thread_key"])
	threadType := models.ThreadTypeUnknown
	if s, ok := mt["thread_type"].(string); ok {
		switch s {
		case "GROUP":
			threadType = models.ThreadTypeGroup
		case "ONE_TO_ONE":
			threadType = models.ThreadTypeUser
		}
	}
	messages, _ := mt["messages"].(map[string]any)
	nodes, _ := messages["nodes"].([]any)
	out := make([]models.Message, 0, len(nodes))
	for _, n := range nodes {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, p.ParseMessageFromGraphQL(m, threadID, threadType))
	}
	return out, nil
}

// ParseMessageFromGraphQL parses one GraphQL message node.
func (p *Parser) ParseMessageFromGraphQL(m map[string]any, threadID string, threadType models.ThreadType) models.Message {
	msgObj, _ := m["message"].(map[string]any)
	sender, _ := m["message_sender"].(map[string]any)
	reactions := []models.MessageReaction{}
	if arr, ok := m["message_reactions"].([]any); ok {
		for _, r := range arr {
			rm, ok := r.(map[string]any)
			if !ok {
				continue
			}
			user, _ := rm["user"].(map[string]any)
			reactions = append(reactions, models.MessageReaction{
				Reaction:             strVal(rm["reaction"]),
				Reactor:              int64(toInt(user["id"])),
				ThreadID:             threadID,
				ReactedMessageSender: int64(toInt(sender["id"])),
				ReactionType:         models.ReactionAdded,
			})
		}
	}
	var attachments []models.Attachment
	if hasGraphQLAttachment(m) {
		if att := p.parseGraphQLAttachment(m); att != nil {
			attachments = []models.Attachment{*att}
		}
	}
	ranges, _ := msgObj["ranges"].([]any)
	return models.Message{
		ID:           strVal(m["message_id"]),
		Text:         strVal(msgObj["text"]),
		SenderID:     strVal(sender["id"]),
		ThreadID:     threadID,
		ThreadType:   threadType,
		MessageType:  messageTypeFromGraphQL(m),
		Timestamp:    int64(toInt(m["timestamp_precise"])),
		CanUnsend:    strings.EqualFold(strVal(m["message_unsendability_status"]), "can_unsend"),
		Unsent:       strVal(m["unsent_timestamp_precise"]) == "0",
		Reactions:    reactions,
		Mentions:     p.parseMentionsFromRanges(ranges),
		ThreadFolder: models.ThreadFolderInbox,
		Attachments:  attachments,
	}
}

// ParseThemes decodes MWPThreadThemeQuery response.
func (p *Parser) ParseThemes(raw []byte) ([]models.Theme, error) {
	cleaned := stripJSONCruft(string(raw))
	var root map[string]any
	if err := json.Unmarshal([]byte(cleaned), &root); err != nil {
		return nil, fberr.Wrap("ParseThemes", "unmarshal", err)
	}
	data, _ := root["data"].(map[string]any)
	tt, _ := data["messenger_thread_themes"].([]any)
	out := make([]models.Theme, 0, len(tt))
	for _, item := range tt {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		gradients := stringSlice(m["gradient_colors"])
		out = append(out, models.Theme{
			ID:                 int64(toInt(m["id"])),
			AccessibilityLabel: strVal(m["accessibility_label"]),
			GradientColors:     gradients,
		})
	}
	return out, nil
}

func (p *Parser) parseClientPayload(delta map[string]any) *ParsedEvent {
	payloadRaw, ok := delta["payload"].([]any)
	if !ok || len(payloadRaw) == 0 {
		return nil
	}
	inner, err := decodeBytePayload(payloadRaw)
	if err != nil || inner == nil {
		return nil
	}
	deltas, _ := inner["deltas"].([]any)
	if len(deltas) == 0 {
		return nil
	}
	first, _ := deltas[0].(map[string]any)
	if reply, ok := deltaMap(first, "deltaMessageReply", "messageReply"); ok {
		etype := events.Message
		if replyType, _ := first["replyType"].(float64); int(replyType) == 1 {
			etype = events.MessageBump
		}
		var replied models.Message
		if rm, ok := reply["repliedToMessage"].(map[string]any); ok {
			replied = p.ParseMessage(rm, nil)
		}
		if mm, ok := reply["message"].(map[string]any); ok {
			main := p.ParseMessage(mm, &replied)
			return &ParsedEvent{EventType: etype, Args: []any{main}}
		}
	}
	if reaction, ok := deltaMap(first, "deltaMessageReaction", "messageReaction"); ok {
		return &ParsedEvent{EventType: events.MessageReaction, Args: []any{parseMessageReaction(reaction)}}
	}
	if unsend, ok := deltaMap(first, "deltaRecallMessageData", "messageUnsend"); ok {
		return &ParsedEvent{EventType: events.MessageUnsent, Args: []any{parseMessageUnsend(unsend)}}
	}
	if remove, ok := deltaMap(first, "deltaRemoveMessage", "messageRemove"); ok {
		return &ParsedEvent{EventType: events.MessageRemove, Args: []any{parseMessageRemove(remove)}}
	}
	if mute, ok := deltaMap(first, "deltaMuteCallsFromThread", "muteThread"); ok {
		return &ParsedEvent{EventType: events.ThreadMute, Args: []any{parseMuteThread(mute)}}
	}
	if page, ok := deltaMap(first, "deltaBiiMPageMessageNotification", "pageNotification"); ok {
		return &ParsedEvent{EventType: events.PageNotification, Args: []any{parsePageNotification(page)}}
	}
	if viewer, ok := deltaMap(first, "deltaChangeViewerStatus", "changeViewerStatus"); ok {
		return &ParsedEvent{EventType: events.ViewerStatusChange, Args: []any{parseChangeViewerStatus(viewer)}}
	}
	return nil
}

func (p *Parser) parseAdminText(delta map[string]any) *ParsedEvent {
	adminType, _ := delta["type"].(string)
	if containsRemoveAdmin(delta["untypedData"]) {
		return nil
	}
	if _, ok := AllAdminText[adminType]; !ok {
		return nil
	}
	etype, ok := adminTextToEvent[adminType]
	if !ok {
		return nil
	}
	decoded, err := decodeAdminUntyped(etype, delta["untypedData"])
	if err != nil {
		return nil
	}
	meta, _ := delta["messageMetadata"].(map[string]any)
	return &ParsedEvent{EventType: etype, Args: []any{decoded, parseMessageData(meta)}}
}

func (p *Parser) parseNotifications(payload []byte) *ParsedEvent {
	raw := decodeNotificationPayload(payload)
	if raw == nil {
		return nil
	}
	switch strVal(raw["type"]) {
	case "friending_state_change":
		return &ParsedEvent{
			EventType: events.FriendRequestChange,
			Args: []any{models.FriendRequestState{
				UserID: strVal(raw["userid"]),
				Action: strVal(raw["action"]),
			}},
		}
	case "mobile_requests_count":
		return &ParsedEvent{
			EventType: events.FriendRequestListUpdate,
			Args: []any{models.FriendRequestList{
				FriendRequests:   []any{raw["num_unread"]},
				NewFriendRequest: toBool(raw["num_unseen"]),
			}},
		}
	case "live_poke":
		return &ParsedEvent{
			EventType: events.PokeNotification,
			Args: []any{models.PokeNotification{
				UserPoked: strVal(raw["poke_source"]),
				PokeTime:  int64(toInt(raw["poke_time"])),
			}},
		}
	default:
		return nil
	}
}

func (p *Parser) parseAttachmentsFromDelta(data map[string]any) []models.Attachment {
	arr, _ := data["attachments"].([]any)
	if len(arr) == 0 {
		return nil
	}
	out := make([]models.Attachment, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		mercury, _ := m["mercury"].(map[string]any)
		if att := p.parseMercury(mercury); att != nil {
			out = append(out, *att)
		}
	}
	return out
}

func (p *Parser) parseMercury(mercury map[string]any) *models.Attachment {
	if mercury == nil {
		return nil
	}
	if sticker, ok := mercury["sticker_attachment"].(map[string]any); ok {
		return &models.Attachment{
			Type: models.AttachmentSticker,
			Data: models.StickerAttachment{ID: strVal(sticker["id"]), URL: strVal(sticker["url"])},
		}
	}
	if blobs, ok := mercury["blob_attachment"].([]any); ok && len(blobs) > 0 {
		blob, _ := blobs[0].(map[string]any)
		typ := models.AttachmentType(strVal(blob["type"]))
		return &models.Attachment{Type: typ, Data: blob}
	}
	if ext, ok := mercury["extensible_attachment"].(map[string]any); ok {
		return p.parseExtensible(ext)
	}
	return nil
}

func (p *Parser) parseExtensible(data map[string]any) *models.Attachment {
	genie := strVal(data["genie_attachment"])
	switch genie {
	case string(models.AttachmentFacebookPost):
		return &models.Attachment{Type: models.AttachmentFacebookPost, Data: p.parsePostExtensible(data)}
	case string(models.AttachmentFacebookReel):
		return &models.Attachment{Type: models.AttachmentFacebookReel, Data: p.parseReelExtensible(data)}
	case string(models.AttachmentFacebookProfile):
		return &models.Attachment{Type: models.AttachmentFacebookProfile, Data: p.parseProfileExtensible(data)}
	case string(models.AttachmentLocation):
		return &models.Attachment{Type: models.AttachmentLocation, Data: p.parseLocationExtensible(data)}
	case string(models.AttachmentFacebookProduct):
		return &models.Attachment{Type: models.AttachmentFacebookProduct, Data: p.parseProductExtensible(data)}
	case string(models.AttachmentExternalURL):
		return &models.Attachment{Type: models.AttachmentExternalURL, Data: p.parseExternalExtensible(data)}
	case "None":
		return &models.Attachment{Type: models.AttachmentFacebookStory, Data: p.parseStoryExtensible(data)}
	default:
		story, _ := data["story_attachment"].(map[string]any)
		target, _ := story["target"].(map[string]any)
		if target != nil {
			typ := models.AttachmentType(strVal(target["__typename"]))
			if mt, ok := attachToMessage[typ]; ok {
				_ = mt
			}
		}
		return &models.Attachment{Type: models.AttachmentExternalURL, Data: p.parseExternalExtensible(data)}
	}
}

func (p *Parser) parseGraphQLAttachment(m map[string]any) *models.Attachment {
	if sticker, ok := m["sticker"].(map[string]any); ok && sticker != nil {
		return &models.Attachment{
			Type: models.AttachmentSticker,
			Data: models.StickerAttachment{ID: strVal(sticker["id"]), URL: strVal(sticker["image"]),},
		}
	}
	if blobs, ok := m["blob_attachments"].([]any); ok && len(blobs) > 0 {
		blob, _ := blobs[0].(map[string]any)
		typ := models.AttachmentType(strVal(blob["__typename"]))
		return &models.Attachment{Type: typ, Data: blob}
	}
	if ext, ok := m["extensible_attachment"].(map[string]any); ok && ext != nil {
		return p.parseExtensible(ext)
	}
	return nil
}

func (p *Parser) parseMentionsFromDelta(data map[string]any) []models.Mention {
	mentions, _ := data["data"].(map[string]any)
	prng, _ := mentions["prng"].([]any)
	return parseMentionList(prng)
}

func (p *Parser) parseMentionsFromRanges(ranges []any) []models.Mention {
	if len(ranges) == 0 {
		return nil
	}
	out := make([]models.Mention, 0, len(ranges))
	for _, r := range ranges {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		entity, _ := m["entity"].(map[string]any)
		out = append(out, models.Mention{
			UserID: strVal(entity["id"]),
			Offset: toInt(m["offset"]),
			Length: toInt(m["length"]),
		})
	}
	return out
}

func parseMentionList(items []any) []models.Mention {
	if len(items) == 0 {
		return nil
	}
	out := make([]models.Mention, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, models.Mention{
			UserID: strVal(m["i"]),
			Offset: toInt(m["o"]),
			Length: toInt(m["l"]),
		})
	}
	return out
}

func messageTypeFromGraphQL(m map[string]any) models.MessageType {
	if blobs, ok := m["blob_attachments"].([]any); ok && len(blobs) > 0 {
		blob, _ := blobs[0].(map[string]any)
		typ := models.AttachmentType(strVal(blob["__typename"]))
		if mt, ok := attachToMessage[typ]; ok {
			return mt
		}
	}
	if sticker, ok := m["sticker"]; ok && sticker != nil {
		return models.MessageTypeSticker
	}
	if ext, ok := m["extensible_attachment"].(map[string]any); ok && ext != nil {
		story, _ := ext["story_attachment"].(map[string]any)
		target, _ := story["target"].(map[string]any)
		if target != nil {
			typ := models.AttachmentType(strVal(target["__typename"]))
			if mt, ok := attachToMessage[typ]; ok {
				return mt
			}
		}
	}
	return models.MessageTypeText
}

func hasGraphQLAttachment(m map[string]any) bool {
	if blobs, ok := m["blob_attachments"].([]any); ok && len(blobs) > 0 {
		return true
	}
	if m["sticker"] != nil {
		return true
	}
	if ext, ok := m["extensible_attachment"]; ok && ext != nil {
		return true
	}
	return false
}

func decodeBytePayload(arr []any) (map[string]any, error) {
	bytesOut := make([]byte, 0, len(arr))
	for _, v := range arr {
		switch n := v.(type) {
		case float64:
			bytesOut = append(bytesOut, byte(int(n)))
		case int:
			bytesOut = append(bytesOut, byte(n))
		}
	}
	var out map[string]any
	if err := json.Unmarshal(bytesOut, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeGraphQLBatch(result any) ([]map[string]any, error) {
	switch v := result.(type) {
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
		return []map[string]any{v}, nil
	default:
		return nil, fberr.Wrap("normalizeGraphQLBatch", "unsupported result type", fberr.ErrParsing)
	}
}

func parseTyping(raw map[string]any) models.Typing {
	return models.Typing{
		SenderID: strVal(raw["sender_fbid"]),
		State:    toInt(raw["state"]),
		ThreadID: strVal(raw["thread"]),
	}
}

func parsePresence(raw map[string]any) models.Presence {
	list, _ := raw["list"].([]any)
	statuses := make([]models.UserStatus, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		statuses = append(statuses, models.UserStatus{
			UserID:     int64(toInt(m["u"])),
			IsActive:   toInt(m["p"]) > 0,
			LastActive: int64(toInt(m["l"])),
		})
	}
	return models.Presence{ListType: strVal(raw["list_type"]), PresenceList: statuses}
}

func parseReadReceipt(delta map[string]any) models.ReadReceipt {
	return models.ReadReceipt{
		UserID:             int64(toInt(delta["actorFbId"])),
		ThreadID:           unwrapStr(delta["threadKey"]),
		Timestamp:          int64(toInt(delta["actionTimestampMs"])),
		WatermarkTimestamp: int64(toInt(delta["watermarkTimestampMs"])),
	}
}

func parseDeliveryReceipt(delta map[string]any) models.DeliveryReceipt {
	ids := []string{}
	if arr, ok := delta["messageIds"].([]any); ok {
		for _, v := range arr {
			ids = append(ids, strVal(v))
		}
	}
	return models.DeliveryReceipt{
		MessageIDs: ids,
		ThreadID:   unwrapStr(delta["threadKey"]),
		UserID:     int64(toInt(unwrapAny(delta["actorFbId"]))),
		Timestamp:  int64(toInt(delta["deliveredWatermarkTimestampMs"])),
	}
}

func parseMarkRead(delta map[string]any) models.MarkRead {
	ids := []string{}
	if arr, ok := delta["threadKeys"].([]any); ok {
		for _, v := range arr {
			ids = append(ids, unwrapStr(v))
		}
	}
	return models.MarkRead{
		ThreadIDs:          ids,
		WatermarkTimestamp: int64(toInt(delta["watermarkTimestamp"])),
		Folder:             unwrapStr(delta["folderId"]),
	}
}

func parseMarkUnread(delta map[string]any) models.MarkUnread {
	ids := []string{}
	if arr, ok := delta["threadKeys"].([]any); ok {
		for _, v := range arr {
			ids = append(ids, unwrapStr(v))
		}
	}
	return models.MarkUnread{ThreadIDs: ids, Timestamp: int64(toInt(delta["actionTimestamp"]))}
}

func parseMessageData(meta map[string]any) models.MessageData {
	if meta == nil {
		return models.MessageData{}
	}
	return models.MessageData{
		ID:         strVal(meta["messageId"]),
		SenderID:   strVal(meta["actorFbId"]),
		Folder:     unwrapStr(meta["folderId"]),
		Timestamp:  int64(toInt(meta["timestamp"])),
		ThreadID:   unwrapStr(meta["threadKey"]),
		AdminText:  strVal(meta["adminText"]),
		UnsendType: strVal(meta["unsendType"]),
	}
}

func parseAdminRemoved(delta map[string]any) models.AdminRemoved {
	meta, _ := delta["messageMetadata"].(map[string]any)
	removed := []string{}
	if arr, ok := delta["removedAdmins"].([]any); ok {
		for _, v := range arr {
			removed = append(removed, unwrapStr(v))
		}
	}
	return models.AdminRemoved{RemovedAdmins: removed, MessageMeta: parseMessageData(meta)}
}

func parseParticipantsAdded(delta map[string]any) models.ParticipantsAdded {
	meta, _ := delta["messageMetadata"].(map[string]any)
	added := []string{}
	if arr, ok := delta["addedParticipants"].([]any); ok {
		for _, v := range arr {
			added = append(added, unwrapStr(v))
		}
	}
	return models.ParticipantsAdded{AddedParticipants: added, MessageMeta: parseMessageData(meta)}
}

func parseParticipantLeft(delta map[string]any) models.ParticipantLeft {
	meta, _ := delta["messageMetadata"].(map[string]any)
	return models.ParticipantLeft{
		LeftParticipant: unwrapStr(delta["leftParticipant"]),
		MessageMeta:     parseMessageData(meta),
	}
}

func parseApprovalMode(delta map[string]any) models.ApprovalMode {
	meta, _ := delta["messageMetadata"].(map[string]any)
	return models.ApprovalMode{Mode: strVal(delta["mode"]), MessageMeta: parseMessageData(meta)}
}

func parseApprovalQueue(delta map[string]any) models.ApprovalQueue {
	meta, _ := delta["messageMetadata"].(map[string]any)
	return models.ApprovalQueue{
		RequesterID: strVal(delta["requesterId"]),
		Action:      strVal(delta["action"]),
		InviterID:   strVal(delta["inviterId"]),
		MessageMeta: parseMessageData(meta),
	}
}

func parseThreadName(delta map[string]any) models.ThreadName {
	meta, _ := delta["messageMetadata"].(map[string]any)
	return models.ThreadName{Name: strVal(delta["name"]), MessageMeta: parseMessageData(meta)}
}

func parseThreadAction(delta map[string]any) models.ThreadAction {
	return models.ThreadAction{Action: strVal(delta["action"]), ThreadID: unwrapStr(delta["threadKey"])}
}

func parseThreadFolderMove(delta map[string]any) models.ThreadFolderMove {
	return models.ThreadFolderMove{
		UserID:   strVal(delta["userId"]),
		Folder:   strVal(delta["folder"]),
		ThreadID: unwrapStr(delta["threadKey"]),
	}
}

func parseThreadDelete(delta map[string]any) models.ThreadDelete {
	ids := []string{}
	if arr, ok := delta["threadIds"].([]any); ok {
		for _, v := range arr {
			ids = append(ids, unwrapStr(v))
		}
	}
	return models.ThreadDelete{UserID: strVal(delta["userId"]), ThreadIDs: ids}
}

func parseThreadMuteSettings(delta map[string]any) models.ThreadMuteSettings {
	return models.ThreadMuteSettings{
		UserID:     strVal(delta["userId"]),
		ThreadID:   unwrapStr(delta["threadKey"]),
		ExpireTime: int64(toInt(delta["expireTimeMs"])),
	}
}

func parseMessageUnsend(data map[string]any) models.MessageUnsend {
	return models.MessageUnsend{
		ID:        strVal(data["messageId"]),
		ThreadID:  unwrapStr(data["threadKey"]),
		SenderID:  strVal(data["senderId"]),
		Timestamp: int64(toInt(data["timestampMs"])),
	}
}

func parseMessageRemove(data map[string]any) models.MessageRemove {
	ids := []string{}
	if arr, ok := data["messageIds"].([]any); ok {
		for _, v := range arr {
			ids = append(ids, strVal(v))
		}
	}
	return models.MessageRemove{IDs: ids, ThreadID: unwrapStr(data["threadKey"])}
}

func parseMuteThread(data map[string]any) models.MuteThread {
	return models.MuteThread{ThreadID: unwrapStr(data["threadKey"]), MuteUntil: int64(toInt(data["muteUntil"]))}
}

func unwrapStr(v any) string {
	for {
		switch x := v.(type) {
		case string:
			return x
		case map[string]any:
			if len(x) == 0 {
				return ""
			}
			v = x[mapFirstKey(x)]
		case float64:
			return fmt.Sprintf("%.0f", x)
		case int:
			return strconv.Itoa(x)
		case json.Number:
			return x.String()
		default:
			if v == nil {
				return ""
			}
			return fmt.Sprint(v)
		}
	}
}

func unwrapAny(v any) any {
	if m, ok := v.(map[string]any); ok && len(m) == 1 {
		return m[mapFirstKey(m)]
	}
	return v
}

func unwrapThreadKey(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return unwrapStr(v)
	}
	if id := strVal(m["thread_fbid"]); id != "" {
		return id
	}
	return strVal(m["other_user_id"])
}

func mapFirstKey(m map[string]any) string {
	for k := range m {
		return k
	}
	return ""
}

func strVal(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int:
		return strconv.Itoa(x)
	case json.Number:
		return x.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
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

func intSlice(v any) []int64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]int64, 0, len(arr))
	for _, item := range arr {
		out = append(out, int64(toInt(item)))
	}
	return out
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		out = append(out, strVal(item))
	}
	return out
}

func containsAny(payload []byte, needles ...[]byte) bool {
	for _, n := range needles {
		if bytes.Contains(payload, n) {
			return true
		}
	}
	return false
}

func stripJSONCruft(content string) string {
	content = strings.TrimSpace(content)
	if idx := strings.Index(content, "{"); idx >= 0 {
		return content[idx:]
	}
	return content
}

func stringify(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func deltaMap(first map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		if m, ok := first[key].(map[string]any); ok {
			return m, true
		}
	}
	return nil, false
}

func decodeNotificationPayload(payload []byte) map[string]any {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil
	}
	if strVal(raw["type"]) != "" {
		return raw
	}
	if arr, ok := raw["deltas"].([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			return m
		}
	}
	return raw
}

func parseMessageReaction(data map[string]any) models.MessageReaction {
	reaction := models.MessageReaction{
		ID:                   strVal(firstStr(data, "messageId", "messageID")),
		ThreadID:             unwrapStr(data["threadKey"]),
		Reactor:              int64(toInt(firstStr(data, "userId", "userID"))),
		ReactedMessageSender: int64(toInt(firstStr(data, "senderId", "senderID"))),
		Reaction:             strVal(data["reaction"]),
		ReactionType:         models.ReactionAction(toInt(data["action"])),
	}
	if ts, ok := data["reactionTimestamp"]; ok {
		v := int64(toInt(ts))
		reaction.Timestamp = &v
	}
	return reaction
}

func parsePageNotification(data map[string]any) models.PageNotification {
	return models.PageNotification{
		SenderID:  strVal(data["senderId"]),
		PageID:    strVal(data["pageId"]),
		PageName:  strVal(data["pageName"]),
		MessageID: strVal(data["messageId"]),
		Title:     strVal(data["title"]),
		Text:      strVal(data["body"]),
	}
}

func parseChangeViewerStatus(data map[string]any) models.ChangeViewerStatus {
	out := models.ChangeViewerStatus{
		UserID:   unwrapStr(data["actorFbid"]),
		ThreadID: unwrapStr(data["threadKey"]),
		CanReply: toBool(data["canViewerReply"]),
		Reason:   toInt(data["reason"]),
	}
	if v, ok := data["isMsgBlockedByViewer"]; ok {
		b := toBool(v)
		out.IsMessengerBlocked = &b
	}
	if v, ok := data["isFBBlockedByViewer"]; ok {
		b := toBool(v)
		out.IsFacebookBlocked = &b
	}
	if v, ok := data["isMsgBlockedTimestamp"]; ok {
		ts := int64(toInt(v))
		out.MessengerBlockedTimestamp = &ts
	}
	if v, ok := data["isFBBlockedTimestamp"]; ok {
		ts := int64(toInt(v))
		out.FacebookBlockedTimestamp = &ts
	}
	return out
}

func firstStr(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}