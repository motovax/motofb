package motofb

import (
	"context"

	"github.com/motovax/motofb/models"
)

// Download saves a remote attachment to disk.
func (c *Client) Download(ctx context.Context, url, filename string) error {
	return c.state.DownloadFile(ctx, url, filename)
}

// --- Messenger API delegates (Python MessengerClient parity) ---

func (c *Client) UploadFiles(ctx context.Context, filePath, fileURL []string, voiceClip, fullData bool) (any, error) {
	return c.messenger.UploadFiles(ctx, filePath, fileURL, voiceClip, fullData)
}

func (c *Client) FetchThreadInfo(ctx context.Context, threadIDs []string) ([]models.Thread, error) {
	return c.messenger.FetchThreadInfo(ctx, threadIDs)
}

func (c *Client) FetchThreadList(ctx context.Context, limit int, folder models.ThreadFolder, before *int64) ([]models.Thread, error) {
	return c.messenger.FetchThreadList(ctx, limit, folder, before)
}

func (c *Client) FetchThreadMessages(ctx context.Context, threadID string, limit int, before *int64) ([]models.Message, error) {
	return c.messenger.FetchThreadMessages(ctx, threadID, limit, before)
}

func (c *Client) FetchAllUsers(ctx context.Context) (map[string]models.User, error) {
	return c.messenger.FetchAllUsers(ctx)
}

func (c *Client) FetchUserInfo(ctx context.Context, userIDs ...string) (map[string]models.User, error) {
	return c.messenger.FetchUserInfo(ctx, userIDs...)
}

func (c *Client) FetchMessageInfo(ctx context.Context, messageID, threadID string) (*models.Message, error) {
	return c.messenger.FetchMessageInfo(ctx, messageID, threadID)
}

func (c *Client) FetchThreadThemes(ctx context.Context) ([]models.Theme, error) {
	return c.messenger.FetchThreadThemes(ctx)
}

func (c *Client) SendMessage(ctx context.Context, text, threadID string, opts ...SendOption) (string, error) {
	cfg := sendConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	var textPtr *string
	if text != "" {
		textPtr = &text
	}
	return c.messenger.SendMessage(ctx, textPtr, threadID, cfg.filePaths, cfg.fileURLs, cfg.sticker, cfg.replyTo, cfg.fileIDs, cfg.mentions)
}

// SendOption configures SendMessage.
type SendOption func(*sendConfig)

type sendConfig struct {
	filePaths []string
	fileURLs  []string
	sticker   *string
	replyTo   *string
	fileIDs   []int64
	mentions  []models.Mention
}

func WithFilePaths(paths []string) SendOption       { return func(c *sendConfig) { c.filePaths = paths } }
func WithFileURLs(urls []string) SendOption         { return func(c *sendConfig) { c.fileURLs = urls } }
func WithSticker(id string) SendOption              { return func(c *sendConfig) { c.sticker = &id } }
func WithReplyTo(messageID string) SendOption       { return func(c *sendConfig) { c.replyTo = &messageID } }
func WithFileIDs(ids []int64) SendOption            { return func(c *sendConfig) { c.fileIDs = ids } }
func WithMentions(m []models.Mention) SendOption    { return func(c *sendConfig) { c.mentions = m } }

func (c *Client) SendQuickReaction(ctx context.Context, threadID, emoji string, size int) error {
	return c.messenger.SendQuickReaction(ctx, threadID, emoji, size)
}

func (c *Client) SendFiles(ctx context.Context, threadID string, fileIDs []int64) error {
	return c.messenger.SendFiles(ctx, threadID, fileIDs)
}

func (c *Client) SendFilesFromPath(ctx context.Context, threadID string, paths []string) error {
	return c.messenger.SendFilesFromPath(ctx, threadID, paths)
}

func (c *Client) SendFilesFromURL(ctx context.Context, threadID string, urls []string) error {
	return c.messenger.SendFilesFromURL(ctx, threadID, urls)
}

func (c *Client) ForwardMessage(ctx context.Context, messageID, toThreadID string) error {
	return c.messenger.ForwardMessage(ctx, messageID, toThreadID)
}

func (c *Client) Unsend(ctx context.Context, messageID, threadID string) error {
	return c.messenger.Unsend(ctx, messageID, threadID)
}

func (c *Client) React(ctx context.Context, reaction, messageID, threadID string) error {
	return c.messenger.React(ctx, reaction, messageID, threadID)
}

func (c *Client) SearchMessage(ctx context.Context, text, threadID string, threadType models.ThreadType) (models.MessageSearchResponse, error) {
	return c.messenger.SearchMessage(ctx, text, threadID, threadType)
}

func (c *Client) PinMessage(ctx context.Context, threadID, messageID string, pin bool) error {
	return c.messenger.PinMessage(ctx, threadID, messageID, pin)
}

func (c *Client) MarkAsRead(ctx context.Context, threadID string) error {
	return c.messenger.MarkAsRead(ctx, threadID)
}

func (c *Client) MarkAsUnread(ctx context.Context, threadID string) error {
	return c.messenger.MarkAsUnread(ctx, threadID)
}

func (c *Client) Typing(ctx context.Context, threadID string, isTyping bool, threadType models.ThreadType) error {
	return c.messenger.Typing(ctx, threadID, isTyping, threadType)
}

func (c *Client) CreateGroupThread(ctx context.Context, participantIDs []string, emoji string) (string, error) {
	return c.messenger.CreateGroupThread(ctx, participantIDs, emoji)
}

func (c *Client) ChangeThreadApproval(ctx context.Context, threadID string, enabled bool) error {
	return c.messenger.ChangeThreadApproval(ctx, threadID, enabled)
}

func (c *Client) ChangeThreadMessageShare(ctx context.Context, threadID string, enabled bool) error {
	return c.messenger.ChangeThreadMessageShare(ctx, threadID, enabled)
}

func (c *Client) ChangeReadReceipts(ctx context.Context, threadID string, enabled bool) error {
	return c.messenger.ChangeReadReceipts(ctx, threadID, enabled)
}

func (c *Client) AddParticipants(ctx context.Context, threadID string, userIDs []int64) error {
	return c.messenger.AddParticipants(ctx, threadID, userIDs)
}

func (c *Client) RemoveParticipant(ctx context.Context, threadID, userID string) error {
	return c.messenger.RemoveParticipant(ctx, threadID, userID)
}

func (c *Client) SetThreadAdmin(ctx context.Context, threadID, userID string, admin bool) error {
	return c.messenger.SetThreadAdmin(ctx, threadID, userID, admin)
}

func (c *Client) ChangeThreadImage(ctx context.Context, threadID string, imageID *int64, imagePath, imageURL *string) error {
	return c.messenger.ChangeThreadImage(ctx, threadID, imageID, imagePath, imageURL)
}

func (c *Client) ChangeThreadName(ctx context.Context, threadID, name string) error {
	return c.messenger.ChangeThreadName(ctx, threadID, name)
}

func (c *Client) ChangeThreadTheme(ctx context.Context, threadID string, themeID int64) error {
	return c.messenger.ChangeThreadTheme(ctx, threadID, themeID)
}

func (c *Client) ChangeThreadEmoji(ctx context.Context, threadID, emoji string) error {
	return c.messenger.ChangeThreadEmoji(ctx, threadID, emoji)
}

func (c *Client) ChangeNickname(ctx context.Context, threadID, userID, nickname string) error {
	return c.messenger.ChangeNickname(ctx, threadID, userID, nickname)
}

func (c *Client) MuteThread(ctx context.Context, threadID string, forever bool, durationMs int64) error {
	return c.messenger.MuteThread(ctx, threadID, forever, durationMs)
}

func (c *Client) RestrictUser(ctx context.Context, userID string, restrict bool) error {
	return c.messenger.RestrictUser(ctx, userID, restrict)
}

func (c *Client) AcceptFriendRequest(ctx context.Context, userID int64) error {
	return c.messenger.AcceptFriendRequest(ctx, userID)
}