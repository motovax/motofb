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

// FacebookNotification is a Facebook.com notification from the realtime gateway.
type FacebookNotification struct {
	NotifID   string
	Body      string
	SenderID  string
	URL       string
	Timestamp int64
	SeenState any
}

// PageNotification for page inbox.
type PageNotification struct {
	SenderID  string `json:"senderId"`
	PageID    string `json:"pageId"`
	PageName  string `json:"pageName"`
	MessageID string `json:"messageId"`
	Title     string `json:"title"`
	Text      string `json:"body"`
}

// Theme for thread themes.
type Theme struct {
	ID                int64
	AccessibilityLabel string
	GradientColors    []string
}

// Thread action deltas (subset — extend as needed).
type AdminAdded struct {
	AddedAdmin string `json:"TARGET_ID"`
	ThreadType string `json:"THREAD_CATEGORY"`
}

// JoinableMode is group link joinability settings.
type JoinableMode struct {
	Mode string `json:"mode"`
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
	ThemeID    string `json:"theme_id"`
	ThemeName  string `json:"theme_name_with_subtitle"`
	ThemeEmoji string `json:"theme_emoji"`
	ThemeColor string `json:"theme_color"`
}

type ThreadEmoji struct {
	Emoji    string `json:"thread_quick_reaction_emoji"`
	EmojiURL string `json:"thread_quick_reaction_emoji_url"`
}

type ThreadNickname struct {
	Nickname      string `json:"nickname"`
	ParticipantID string `json:"participant_id"`
}

type ThreadMagicWord struct {
	MagicWord             string `json:"magic_word"`
	Emoji                 string `json:"emoji_effect"`
	NewMagicWordCount     string `json:"new_magic_word_count"`
	RemovedMagicWordCount string `json:"removed_magic_word_count"`
	ThemeName             string `json:"theme_name"`
}

type ThreadMessagePin struct {
	MessageID string `json:"pinned_message_id"`
}

type ThreadMessageUnpin struct {
	MessageID string `json:"pinned_message_id"`
}

type ThreadMessageSharing struct {
	Mode       string `json:"limit_sharing_type"`
	SenderName string `json:"sender_name"`
	SenderID   string `json:"sender_id"`
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
	UserID                    string `json:"actorFbid"`
	ThreadID                  string `json:"threadKey"`
	CanReply                  bool   `json:"canViewerReply"`
	Reason                    int    `json:"reason"`
	IsMessengerBlocked        *bool  `json:"isMsgBlockedByViewer"`
	MessengerBlockedTimestamp *int64 `json:"isMsgBlockedTimestamp"`
	IsFacebookBlocked         *bool  `json:"isFBBlockedByViewer"`
	FacebookBlockedTimestamp  *int64 `json:"isFBBlockedTimestamp"`
}