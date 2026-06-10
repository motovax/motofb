package models

// Thread is Messenger conversation metadata.
type Thread struct {
	Name                  string
	ThreadID              string
	MessageCount          int
	Image                 string
	ThreadType            ThreadType
	Folder                ThreadFolder
	ParticipantsNickname  map[string]string
	ThreadAdmins          []string
	PrivacyMode           int
	ApprovalMode          int
	GroupApprovalQueue    []JoinRequest
	JoinableMode          int
	JoinableLink          string
	IsJoined              bool
	IsPinned              bool
	AllParticipants       []User
	Description           string
	ThreadTheme           map[string]any
	PinnedMessages        []any
}

// JoinRequest is a pending group join.
type JoinRequest struct {
	Requester         string
	Inviter           string
	RequestTimestamp  int64
}