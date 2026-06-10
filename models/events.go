package models

// Typing indicator event.
type Typing struct {
	SenderID string
	State    int
	ThreadID string
}

// UserStatus in presence list.
type UserStatus struct {
	UserID     int64
	IsActive   bool
	LastActive int64
}

// Presence update event.
type Presence struct {
	ListType    string
	PresenceList []UserStatus
}

// ReadReceipt when message seen.
type ReadReceipt struct {
	UserID              int64
	ThreadID            string
	Timestamp           int64
	WatermarkTimestamp  int64
}

// DeliveryReceipt when message delivered.
type DeliveryReceipt struct {
	MessageIDs []string
	ThreadID   string
	UserID     int64
	Timestamp  int64
}

// MarkRead event.
type MarkRead struct {
	ThreadIDs          []string
	WatermarkTimestamp int64
	Folder             string
}

// MarkUnread event.
type MarkUnread struct {
	ThreadIDs []string
	Timestamp int64
}

// MessageData is metadata on admin/system messages.
type MessageData struct {
	ID         string
	SenderID   string
	Folder     string
	Timestamp  int64
	ThreadID   string
	AdminText  string
	UnsendType string
}

// FriendRequestState notification.
type FriendRequestState struct {
	UserID string
	Action string
}

// FriendRequestList update.
type FriendRequestList struct {
	FriendRequests    []any
	NewFriendRequest  bool
}

// PokeNotification event.
type PokeNotification struct {
	UserPoked string
	PokeTime  int64
}

// PageNotification for page inbox.
type PageNotification struct {
	SenderID  string
	PageID    string
	PageName  string
	MessageID string
	Text      string
}

// Theme for thread themes.
type Theme struct {
	ID                int64
	AccessibilityLabel string
	GradientColors    []string
}

// Thread action deltas (subset — extend as needed).
type AdminAdded struct {
	AddedAdmin string
	ThreadType ThreadType
}

type AdminRemoved struct {
	RemovedAdmins []string
	MessageMeta   MessageData
}

type ApprovalMode struct {
	Mode        string
	MessageMeta MessageData
}

type ApprovalQueue struct {
	RequesterID string
	Action      string
	InviterID   string
	MessageMeta MessageData
}

type ParticipantsAdded struct {
	AddedParticipants []string
	MessageMeta       MessageData
	Participants      []int64
}

type ParticipantLeft struct {
	LeftParticipant string
	MessageMeta     MessageData
}

type ThreadName struct {
	Name        string
	MessageMeta MessageData
}

type ThreadTheme struct {
	ThemeID    int64
	ThemeName  string
	ThemeEmoji string
	ThemeColor string
}

type ThreadEmoji struct {
	Emoji    string
	EmojiURL string
}

type ThreadNickname struct {
	Nickname      string
	ParticipantID string
}

type ThreadMagicWord struct {
	MagicWord              string
	Emoji                  string
	NewMagicWordCount      string
	RemovedMagicWordCount  string
	ThemeName              string
}

type ThreadMessagePin struct {
	MessageID string
}

type ThreadMessageUnpin struct {
	MessageID string
}

type ThreadMessageSharing struct {
	Mode       string
	SenderName string
	SenderID   string
}

type ThreadMuteSettings struct {
	UserID     string
	ThreadID   string
	ExpireTime int64
}

type MuteThread struct {
	ThreadID  string
	MuteUntil int64
}

type ThreadAction struct {
	Action   string
	ThreadID string
}

type ThreadFolderMove struct {
	UserID   string
	Folder   string
	ThreadID string
}

type ThreadDelete struct {
	UserID    string
	ThreadIDs []string
}

type ChangeViewerStatus struct {
	UserID              string
	ThreadID            string
	CanReply            bool
	IsMessengerBlocked  bool
	IsFacebookBlocked   bool
}