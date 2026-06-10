package messenger

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/motovax/motofb/graphql"
	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/internal"
	"github.com/motovax/motofb/models"
	"github.com/motovax/motofb/parser"
	"github.com/motovax/motofb/state"
)

// Service implements MessengerClient operations over HTTP and /ls_req.
type Service struct {
	State  *state.State
	UID    string
	LS     *LSRequester
	Parser *parser.Parser
}

// NewService constructs a messenger service.
func NewService(st *state.State, uid string, ls *LSRequester, p *parser.Parser) *Service {
	if p == nil {
		p = parser.New()
	}
	return &Service{State: st, UID: uid, LS: ls, Parser: p}
}

func (s *Service) requireState() error {
	if s.State == nil || !s.State.LoggedIn {
		return state.ErrNotLoggedIn()
	}
	return nil
}

func threadInt(threadID string) int {
	n, _ := strconv.Atoi(threadID)
	return n
}

// UploadFiles uploads local paths or remote URLs and returns Mercury file ids.
// When fullData is true, returns []state.UploadFileResult instead of []int64.
func (s *Service) UploadFiles(ctx context.Context, filePath, fileURL []string, voiceClip, fullData bool) (any, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	var files []state.FilePart
	var err error
	switch {
	case len(filePath) > 0:
		files, err = state.FilesFromPaths(filePath)
		if err != nil {
			return nil, err
		}
	case len(fileURL) > 0:
		files, err = state.FilesFromURLs(ctx, s.State.HTTP, fileURL)
		if err != nil {
			return nil, err
		}
		voiceClip = true
	default:
		return nil, fberr.Wrap("UploadFiles", "'file_path' or 'file_url' must be provided", fberr.ErrValidation)
	}
	if fullData {
		return s.State.UploadFilesDetailed(ctx, files, voiceClip)
	}
	return s.State.UploadFiles(ctx, files, voiceClip)
}

func (s *Service) uploadFileIDs(ctx context.Context, filePath, fileURL []string, voiceClip bool) ([]int64, error) {
	raw, err := s.UploadFiles(ctx, filePath, fileURL, voiceClip, false)
	if err != nil {
		return nil, err
	}
	ids, ok := raw.([]int64)
	if !ok {
		return nil, fberr.New("uploadFileIDs", "unexpected upload result type")
	}
	return ids, nil
}

// FetchThreadInfo fetches metadata for multiple threads.
func (s *Service) FetchThreadInfo(ctx context.Context, threadIDs []string) ([]models.Thread, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	queries := make([]graphql.QueryRequest, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		queries = append(queries, graphql.FromDocID(graphql.DocThreadFetcher, map[string]any{
			"id":                  threadID,
			"message_limit":       0,
			"load_messages":       false,
			"load_read_receipts":  false,
			"before":              nil,
		}))
	}
	result, err := s.State.GraphQLBatchNamed(ctx, "MessengerGraphQLThreadFetcher", queries...)
	if err != nil {
		return nil, fberr.Wrap("FetchThreadInfo", "failed to fetch thread", err)
	}
	return models.ParseThreads(result)
}

// FetchThreadList fetches threads from a folder.
func (s *Service) FetchThreadList(ctx context.Context, limit int, folder models.ThreadFolder, before *int64) ([]models.Thread, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	params := map[string]any{
		"limit":                   limit,
		"tags":                    string(folder),
		"before":                  before,
		"includeDeliveryReceipts": true,
		"includeSeqID":            false,
	}
	result, err := s.State.GraphQLBatchNamed(ctx, "MessengerGraphQLThreadlistFetcher", graphql.FromDocID(graphql.DocThreadList, params))
	if err != nil {
		return nil, fberr.Wrap("FetchThreadList", "failed to fetch thread folder", err)
	}
	if len(result) == 0 {
		return nil, nil
	}
	viewer, _ := result[0]["viewer"].(map[string]any)
	mt, _ := viewer["message_threads"].(map[string]any)
	return models.ParseThreads(mt["nodes"])
}

// FetchThreadMessages fetches messages from a thread.
func (s *Service) FetchThreadMessages(ctx context.Context, threadID string, messageLimit int, before *int64) ([]models.Message, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	params := map[string]any{
		"id":                 threadID,
		"message_limit":      messageLimit,
		"load_messages":      true,
		"load_read_receipts": true,
		"before":             nil,
	}
	if before != nil {
		params["before"] = *before
	}
	result, err := s.State.GraphQLBatch(ctx, graphql.FromDocID(graphql.DocThreadMessages, params))
	if err != nil {
		return nil, err
	}
	return s.Parser.ParseThreadMessage(result)
}

// FetchAllUsers fetches all user threads the client is chatting with.
func (s *Service) FetchAllUsers(ctx context.Context) (map[string]models.User, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	raw, err := s.State.PostRaw(ctx, "/chat/user_info_all", map[string]string{"viewer": s.UID})
	if err != nil {
		return nil, fberr.Wrap("FetchAllUsers", "failed to fetch all users info", err)
	}
	payload := trimToJSON(raw)
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fberr.Wrap("FetchAllUsers", "decode response", err)
	}
	return models.ParseUsersFromGraphQL(decoded), nil
}

// FetchUserInfo fetches user profiles by id.
func (s *Service) FetchUserInfo(ctx context.Context, userIDs ...string) (map[string]models.User, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	data := map[string]string{}
	for i, userID := range userIDs {
		data["ids["+strconv.Itoa(i)+"]"] = userID
	}
	raw, err := s.State.PostRaw(ctx, "/chat/user_info/", data)
	if err != nil {
		return nil, fberr.Wrap("FetchUserInfo", "failed to fetch users info", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(trimToJSON(raw), &decoded); err != nil {
		return nil, fberr.Wrap("FetchUserInfo", "decode response", err)
	}
	return models.ParseUsersFromGraphQL(decoded), nil
}

// FetchMessageInfo fetches a single message by id.
func (s *Service) FetchMessageInfo(ctx context.Context, messageID, threadID string) (*models.Message, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	result, err := s.State.GraphQLBatch(ctx, graphql.FromDocID(graphql.DocMessageInfo, map[string]any{
		"thread_and_message_id": map[string]any{
			"thread_id":  threadID,
			"message_id": messageID,
		},
	}))
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	msgObj, _ := result[0]["message"].(map[string]any)
	if msgObj == nil {
		return nil, nil
	}
	msg := s.Parser.ParseMessageFromGraphQL(msgObj, threadID, models.ThreadTypeUnknown)
	return &msg, nil
}

// FetchThreadThemes fetches available thread themes.
func (s *Service) FetchThreadThemes(ctx context.Context) ([]models.Theme, error) {
	if err := s.requireState(); err != nil {
		return nil, err
	}
	data := map[string]string{
		"fb_api_caller_class":        "RelayModern",
		"fb_api_req_friendly_name":   "MWPThreadThemeQuery_AllThemesQuery",
		"variables":                  `{"version":"default"}`,
		"server_timestamps":          "true",
		"doc_id":                     graphql.DocThreadThemes,
	}
	raw, err := s.State.PostGraphQLSingle(ctx, data)
	if err != nil {
		return nil, err
	}
	return s.Parser.ParseThemes(raw)
}

// SendMessage sends text, sticker, files, or reply to a thread.
func (s *Service) SendMessage(ctx context.Context, text *string, threadID string, filePath, fileURL []string, sticker, replyTo *string, fileIDs []int64, mentions []models.Mention) (string, error) {
	if err := s.requireState(); err != nil {
		return "", err
	}
	payload := map[string]any{
		"thread_id":            threadInt(threadID),
		"otid":                 internal.GenerateOfflineThreadingID(),
		"source":               0,
		"send_type":            1,
		"sync_group":           1,
		"text":                 text,
		"initiating_source":    1,
		"skip_url_preview_gen": 0,
	}
	if len(mentions) > 0 {
		payload["mention_data"] = models.MentionsPayload{Users: mentions}.ToMap()
	}
	if sticker != nil && *sticker != "" {
		payload["send_type"] = 2
		payload["sticker_id"] = *sticker
		payload["text"] = nil
	}
	if len(fileIDs) > 0 {
		payload["send_type"] = 3
		payload["attachment_fbids"] = fileIDs
	}
	if len(filePath) > 0 {
		payload["send_type"] = 3
		ids, err := s.uploadFileIDs(ctx, filePath, nil, true)
		if err != nil {
			return "", err
		}
		payload["attachment_fbids"] = ids
	} else if len(fileURL) > 0 {
		payload["send_type"] = 3
		ids, err := s.uploadFileIDs(ctx, nil, fileURL, true)
		if err != nil {
			return "", err
		}
		payload["attachment_fbids"] = ids
	}
	if replyTo != nil && *replyTo != "" {
		payload["reply_metadata"] = map[string]any{
			"reply_source_id":   *replyTo,
			"reply_source_type": 1,
			"reply_type":        0,
		}
	}
	tasks := []Task{
		{Label: "46", Payload: payload, QueueName: threadID},
		{Label: "21", Payload: map[string]any{
			"thread_id":              threadInt(threadID),
			"last_read_watermark_ts": internal.NowMillis(),
			"sync_group":               1,
		}, QueueName: threadID},
	}
	resp, err := s.LS.SendTasks(AppMessengerPrimary, VerSendMessage, tasks)
	if err != nil {
		return "", err
	}
	if strings.Contains(resp.Payload, "Couldn't send") {
		return "", nil
	}
	return ExtractMessageID(resp.Payload), nil
}

// SendQuickReaction sends a tapback-style emoji.
func (s *Service) SendQuickReaction(ctx context.Context, threadID, emoji string, emojiSize int) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	if emojiSize > 3 {
		emojiSize = 3
	} else if emojiSize < 1 {
		emojiSize = 1
	}
	otid := internal.GenerateOfflineThreadingID()
	return s.LS.PublishTasks(AppMessengerLS, VerLSDefault, []Task{
		{Label: "46", Payload: map[string]any{
			"thread_id":            threadInt(threadID),
			"otid":                 otid,
			"source":               65537,
			"send_type":            1,
			"sync_group":           1,
			"mark_thread_read":     1,
			"text":                 emoji,
			"hot_emoji_size":       emojiSize,
			"initiating_source":    1,
			"skip_url_preview_gen": 0,
			"text_has_links":       0,
			"multitab_env":         0,
		}, QueueName: threadID},
		{Label: "21", Payload: map[string]any{
			"thread_id":              threadInt(threadID),
			"last_read_watermark_ts": internal.NowMillis(),
			"sync_group":               1,
		}, QueueName: threadID},
	})
}

// SendFiles sends already-uploaded file ids.
func (s *Service) SendFiles(ctx context.Context, threadID string, fileIDs []int64) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	return s.LS.PublishTasks(AppMessengerLS, VerLSDefault, []Task{{
		Label: "46",
		Payload: map[string]any{
			"thread_id":        threadInt(threadID),
			"otid":             internal.GenerateOfflineThreadingID(),
			"source":           65537,
			"send_type":        3,
			"sync_group":       1,
			"mark_thread_read": 0,
			"text":             nil,
			"attachment_fbids": fileIDs,
		},
		QueueName: threadID,
	}})
}

// SendFilesFromPath uploads and sends local files.
func (s *Service) SendFilesFromPath(ctx context.Context, threadID string, filePaths []string) error {
	ids, err := s.uploadFileIDs(ctx, filePaths, nil, true)
	if err != nil {
		return err
	}
	return s.SendFiles(ctx, threadID, ids)
}

// SendFilesFromURL uploads and sends remote files.
func (s *Service) SendFilesFromURL(ctx context.Context, threadID string, fileURLs []string) error {
	ids, err := s.uploadFileIDs(ctx, nil, fileURLs, true)
	if err != nil {
		return err
	}
	return s.SendFiles(ctx, threadID, ids)
}

// ForwardMessage forwards a message to another thread.
func (s *Service) ForwardMessage(ctx context.Context, messageID, forwardThreadID string) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	return s.LS.PublishTasks(AppMessengerLS, VerForwardSearch, []Task{{
		Label: "46",
		Payload: map[string]any{
			"thread_id":                   threadInt(forwardThreadID),
			"otid":                        internal.GenerateOfflineThreadingID(),
			"source":                      65537,
			"send_type":                   5,
			"sync_group":                  1,
			"mark_thread_read":            0,
			"forwarded_msg_id":            messageID,
			"strip_forwarded_msg_caption": 0,
			"initiating_source":           1,
			"text":                        nil,
		},
		QueueName: forwardThreadID,
	}})
}

// Unsend removes a message for everyone.
func (s *Service) Unsend(ctx context.Context, messageID, threadID string) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	return s.LS.PublishTasks(AppMessengerLS, VerUnsendUnread, []Task{{
		Label: "33",
		Payload: map[string]any{
			"message_id": messageID,
			"thread_key": threadInt(threadID),
			"sync_group": 1,
		},
		QueueName: "unsend_message",
	}})
}

// React adds a reaction to a message.
func (s *Service) React(ctx context.Context, reaction, messageID, threadID string) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	queue, _ := json.Marshal([]string{"reaction", messageID})
	return s.LS.PublishTasks(AppMessengerLS, VerReactCreateGroup, []Task{{
		Label: "29",
		Payload: map[string]any{
			"thread_key":         threadInt(threadID),
			"timestamp_ms":       internal.NowMillis(),
			"message_id":         messageID,
			"actor_id":           s.UID,
			"reaction":           reaction,
			"reaction_style":     nil,
			"sync_group":         1,
			"send_attribution":   65537,
			"dataclass_params": nil,
			"attachment_fbid":  nil,
		},
		QueueName: string(queue),
	}})
}

// SearchMessage searches for text in a thread.
func (s *Service) SearchMessage(ctx context.Context, text, threadID string, threadType models.ThreadType) (models.MessageSearchResponse, error) {
	if err := s.requireState(); err != nil {
		return models.MessageSearchResponse{}, err
	}
	_ = ctx
	resp, err := s.LS.SendTasks(AppMessengerLS, VerForwardSearch, []Task{{
		Label: "107",
		Payload: map[string]any{
			"query":            text,
			"type":             int(threadType),
			"thread_key":       threadInt(threadID),
			"next_page_cursor": nil,
		},
		QueueName: "message_search",
	}})
	if err != nil {
		return models.MessageSearchResponse{}, err
	}
	return models.ParseMessageSearch(resp.Payload), nil
}

// PinMessage pins or unpins a message.
func (s *Service) PinMessage(ctx context.Context, threadID, messageID string, pin bool) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	stateVal := 0
	if pin {
		stateVal = 1
	}
	return s.LS.PublishTasks(AppMessengerLS, VerLSDefault, []Task{{
		Label: "751",
		Payload: map[string]any{
			"thread_key":             threadInt(threadID),
			"message_id":             messageID,
			"pinned_message_state": stateVal,
		},
		QueueName: "set_pinned_message_search",
	}})
}

// MarkAsRead marks a thread as read.
func (s *Service) MarkAsRead(ctx context.Context, threadID string) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	return s.LS.PublishTasks(AppMessengerLS, VerMarkRead, []Task{{
		Label: "21",
		Payload: map[string]any{
			"thread_id":              threadInt(threadID),
			"last_read_watermark_ts": internal.NowMillis() + 5000,
			"sync_group":               1,
		},
		QueueName: threadID,
	}})
}

// MarkAsUnread marks a thread as unread.
func (s *Service) MarkAsUnread(ctx context.Context, threadID string) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	return s.LS.PublishTasks(AppMessengerLS, VerUnsendUnread, []Task{{
		Label: "49",
		Payload: map[string]any{
			"thread_key":                       threadInt(threadID),
			"last_read_watermark_timestamp_ms": internal.NowMillis(),
			"sync_group":                       1,
		},
		QueueName: threadID,
	}})
}

// Typing toggles the typing indicator.
func (s *Service) Typing(ctx context.Context, threadID string, isTyping bool, threadType models.ThreadType) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	isGroup := threadType != models.ThreadTypeUser
	return s.LS.PublishTyping(s.UID, threadID, isGroup, isTyping, int(threadType))
}

// CreateGroupThread creates a group chat and returns the new thread id.
func (s *Service) CreateGroupThread(ctx context.Context, participantIDs []string, emojiSticker string) (string, error) {
	if err := s.requireState(); err != nil {
		return "", err
	}
	_ = ctx
	if emojiSticker == "" {
		emojiSticker = "369239263222822"
	}
	clientThreadKey := internal.GenerateOfflineThreadingID()
	first := Task{
		Label: "153",
		Payload: map[string]any{
			"participants":      participantIDs,
			"client_thread_key": internal.GenerateOfflineThreadingID(),
			"sync_group":        1,
		},
		QueueName: clientThreadKey,
	}
	if err := s.LS.PublishTasks(AppMessengerLS, VerReactCreateGroup, []Task{first}); err != nil {
		return "", err
	}
	second := Task{
		Label: "130",
		Payload: map[string]any{
			"participants": participantIDs,
			"send_payload": map[string]any{
				"thread_id":            clientThreadKey,
				"otid":                 internal.GenerateOfflineThreadingID(),
				"source":               65537,
				"send_type":            2,
				"sync_group":           1,
				"mark_thread_read":     0,
				"sticker_id":           emojiSticker,
				"hot_emoji_size":       1,
				"initiating_source":    1,
				"skip_url_preview_gen": 0,
				"text_has_links":       0,
				"multitab_env":         0,
			},
			"thread_metadata": nil,
		},
		QueueName: clientThreadKey,
	}
	resp, err := s.LS.SendTasks(AppMessengerLS, VerReactCreateGroup, []Task{second})
	if err != nil {
		return "", err
	}
	return ExtractThreadID(resp.Payload), nil
}

// ChangeThreadApproval toggles join approval mode.
func (s *Service) ChangeThreadApproval(ctx context.Context, threadID string, enabled bool) error {
	return s.publishThreadSetting(ctx, "28", "set_needs_admin_approval_for_new_participant", threadID, map[string]any{
		"thread_key": threadInt(threadID),
		"enabled":    bool01(enabled),
		"sync_group": 1,
	})
}

// ChangeThreadMessageShare toggles message sharing permission.
func (s *Service) ChangeThreadMessageShare(ctx context.Context, threadID string, enabled bool) error {
	return s.publishThreadSetting(ctx, "210002", "limit_sharing_setting", threadID, map[string]any{
		"thread_key":               threadInt(threadID),
		"is_limit_sharing_enabled": bool01(enabled),
		"sync_group":               1,
	})
}

// ChangeReadReceipts toggles read receipts for a group thread.
func (s *Service) ChangeReadReceipts(ctx context.Context, threadID string, enabled bool) error {
	return s.publishThreadSetting(ctx, "60003", threadID, threadID, map[string]any{
		"thread_key":                  threadInt(threadID),
		"is_read_receipts_disabled": bool01(enabled),
		"sync_group":                  1,
	})
}

// AddParticipants adds users to a group.
func (s *Service) AddParticipants(ctx context.Context, threadID string, userIDs []int64) error {
	return s.publishThreadSetting(ctx, "23", threadID, threadID, map[string]any{
		"thread_key":  threadInt(threadID),
		"contact_ids": userIDs,
		"sync_group":  1,
	})
}

// RemoveParticipant removes a user from a group.
func (s *Service) RemoveParticipant(ctx context.Context, threadID, userID string) error {
	return s.publishThreadSetting(ctx, "140", "remove_participant_v2", threadID, map[string]any{
		"thread_id":  threadInt(threadID),
		"contact_id": threadInt(userID),
		"sync_group": 1,
	})
}

// SetThreadAdmin grants or revokes admin privileges.
func (s *Service) SetThreadAdmin(ctx context.Context, threadID, userID string, isAdmin bool) error {
	return s.publishThreadSetting(ctx, "25", "admin_status", threadID, map[string]any{
		"thread_key": threadInt(threadID),
		"contact_id": threadInt(userID),
		"is_admin":   bool01(isAdmin),
		"sync_group": 1,
	})
}

// ChangeThreadImage updates a group photo.
func (s *Service) ChangeThreadImage(ctx context.Context, threadID string, imageID *int64, imagePath, imageURL *string) error {
	if err := s.requireState(); err != nil {
		return err
	}
	var id int64
	switch {
	case imageID != nil:
		id = *imageID
	case imagePath != nil && *imagePath != "":
		ids, err := s.uploadFileIDs(ctx, []string{*imagePath}, nil, true)
		if err != nil {
			return err
		}
		id = ids[0]
	case imageURL != nil && *imageURL != "":
		ids, err := s.uploadFileIDs(ctx, nil, []string{*imageURL}, true)
		if err != nil {
			return err
		}
		id = ids[0]
	}
	return s.publishThreadSetting(ctx, "37", "thread_image", threadID, map[string]any{
		"thread_key": threadInt(threadID),
		"image_id":   id,
		"sync_group": 1,
	})
}

// ChangeThreadName renames a group chat.
func (s *Service) ChangeThreadName(ctx context.Context, threadID, name string) error {
	return s.publishThreadSetting(ctx, "32", threadID, threadID, map[string]any{
		"thread_key":  threadInt(threadID),
		"thread_name": name,
		"sync_group":  1,
	})
}

// ChangeThreadTheme updates a thread theme.
func (s *Service) ChangeThreadTheme(ctx context.Context, threadID string, themeID int64) error {
	return s.publishThreadSetting(ctx, "43", "thread_theme", threadID, map[string]any{
		"thread_key": threadInt(threadID),
		"theme_fbid": themeID,
		"source":     nil,
		"sync_group": 1,
		"payload":    nil,
	})
}

// ChangeThreadEmoji sets the quick reaction emoji.
func (s *Service) ChangeThreadEmoji(ctx context.Context, threadID, emoji string) error {
	return s.publishThreadSetting(ctx, "100003", "thread_quick_reaction", threadID, map[string]any{
		"thread_key":   threadInt(threadID),
		"custom_emoji": emoji,
		"sync_group":   1,
	})
}

// ChangeNickname sets a participant nickname.
func (s *Service) ChangeNickname(ctx context.Context, threadID, userID, nickname string) error {
	return s.publishThreadSetting(ctx, "44", "thread_participant_nickname", threadID, map[string]any{
		"thread_key": threadInt(threadID),
		"contact_id": threadInt(userID),
		"nickname":   nickname,
		"sync_group": 1,
	})
}

// MuteThread mutes messages and calls in a thread.
func (s *Service) MuteThread(ctx context.Context, threadID string, muteForever bool, durationMs int64) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	expire := durationMs
	if muteForever {
		expire = -1
	}
	return s.LS.PublishTasks(AppMessengerLS, VerLSDefault, []Task{
		{Label: "144", Payload: map[string]any{
			"thread_key":          threadInt(threadID),
			"mailbox_type":        0,
			"mute_expire_time_ms": expire,
			"sync_group":          1,
		}, QueueName: threadID},
		{Label: "229", Payload: map[string]any{
			"thread_key":                threadInt(threadID),
			"mailbox_type":              0,
			"mute_calls_expire_time_ms": expire,
			"request_id":                nil,
			"sync_group":                1,
		}, QueueName: threadID},
	})
}

// RestrictUser restricts or unrestricts a user.
func (s *Service) RestrictUser(ctx context.Context, userID string, restrict bool) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	action := 1
	if restrict {
		action = 0
	}
	reqID := internal.GenerateUUID()
	if err := s.LS.PublishTasks(AppMessengerLS, VerLSDefault, []Task{{
		Label: "367",
		Payload: map[string]any{
			"restrictee_id":             threadInt(userID),
			"request_id":                reqID,
			"messenger_restrict_action": action,
		},
		QueueName: "messenger_restrict",
	}}); err != nil {
		return err
	}
	if restrict {
		return s.LS.PublishTasks(AppMessengerLS, VerLSDefault, []Task{{
			Label: "810",
			Payload: map[string]any{
				"thread_key": threadInt(userID),
			},
			QueueName: "remove_pinned_thread_on_restrict",
		}})
	}
	return nil
}

// AcceptFriendRequest accepts a friend request.
func (s *Service) AcceptFriendRequest(ctx context.Context, userID int64) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	return s.LS.PublishTasks(AppMessengerLS, VerLSDefault, []Task{{
		Label: "207",
		Payload: map[string]any{
			"contact_id": userID,
		},
		QueueName: "cpq_v2",
	}})
}

func (s *Service) publishThreadSetting(ctx context.Context, label, queueName, _ string, payload map[string]any) error {
	if err := s.requireState(); err != nil {
		return err
	}
	_ = ctx
	version := VerThreadSettings
	if label == "37" || label == "32" || label == "43" || label == "100003" {
		version = VerReactCreateGroup
	}
	if label == "44" {
		version = VerLSDefault
	}
	return s.LS.PublishTasks(AppMessengerLS, version, []Task{{
		Label:     label,
		Payload:   payload,
		QueueName: queueName,
	}})
}

func trimToJSON(raw []byte) []byte {
	idx := bytesIndex(raw, '{')
	if idx < 0 {
		return raw
	}
	return raw[idx:]
}

func bytesIndex(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}