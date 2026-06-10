// Package models defines Facebook Messenger domain types.
package models

// MessageType classifies message content.
type MessageType string

const (
	MessageTypeText            MessageType = "text"
	MessageTypeImage           MessageType = "image"
	MessageTypeVideo           MessageType = "video"
	MessageTypeAudio           MessageType = "audio"
	MessageTypeGIF             MessageType = "animated_image"
	MessageTypeSticker         MessageType = "sticker"
	MessageTypeFile            MessageType = "file"
	MessageTypeLocation        MessageType = "location"
	MessageTypeFacebookPost    MessageType = "post"
	MessageTypeFacebookProfile MessageType = "profile"
	MessageTypeFacebookReel    MessageType = "reel"
	MessageTypeFacebookProduct MessageType = "product"
	MessageTypeExternalURL     MessageType = "external_url"
	MessageTypeAdminText       MessageType = "admin_text"
)

// ThreadType identifies conversation kind.
type ThreadType int

const (
	ThreadTypeUser      ThreadType = 1
	ThreadTypeGroup     ThreadType = 2
	ThreadTypePage      ThreadType = 3
	ThreadTypeCommunity ThreadType = 2
	ThreadTypeUnknown   ThreadType = 3
)

// ThreadFolder is inbox placement.
type ThreadFolder string

const (
	ThreadFolderInbox     ThreadFolder = "INBOX"
	ThreadFolderArchive   ThreadFolder = "ARCHIVE"
	ThreadFolderPending   ThreadFolder = "PENDING"
	ThreadFolderSpam      ThreadFolder = "SPAM"
	ThreadFolderCommunity ThreadFolder = "COMMUNITY"
	ThreadFolderE2EE      ThreadFolder = "E2EE_CUTOVER"
	ThreadFolderOther     ThreadFolder = "OTHER"
)

// ReactionAction is add/remove reaction.
type ReactionAction int

const (
	ReactionAdded   ReactionAction = 0
	ReactionRemoved ReactionAction = 1
)

// AttachmentType for parsers.
type AttachmentType string

const (
	AttachmentImage           AttachmentType = "ImageAttachment"
	AttachmentVideo           AttachmentType = "VideoAttachment"
	AttachmentGIF             AttachmentType = "AnimatedImageAttachment"
	AttachmentSticker         AttachmentType = "StickerAttachment"
	AttachmentFile            AttachmentType = "FileAttachment"
	AttachmentAudio           AttachmentType = "AudioAttachment"
	AttachmentLocation        AttachmentType = "LocationAttachment"
	AttachmentFacebookPost    AttachmentType = "StoryAttachment"
	AttachmentFacebookReel    AttachmentType = "FBReelShareAttachment"
	AttachmentFacebookProfile AttachmentType = "ProfileAttachment"
	AttachmentFacebookProduct AttachmentType = "ProductAttachment"
	AttachmentExternalURL   AttachmentType = "ExternalUrlAttachment"
	AttachmentFacebookStory   AttachmentType = "StoryShareAttachment"
	AttachmentFacebookGame    AttachmentType = "GameShareAttachment"
)

// FBReaction IDs for post reactions.
type FBReaction string

const (
	FBReactionLike  FBReaction = "1635855486666999"
	FBReactionLove  FBReaction = "1678524932434102"
	FBReactionCare  FBReaction = "613557422527858"
	FBReactionHaha  FBReaction = "115940658764963"
	FBReactionWow   FBReaction = "478547315650144"
	FBReactionSad   FBReaction = "908563459236466"
	FBReactionAngry FBReaction = "444813342392137"
)

// Audience for Facebook posts.
type Audience string

const (
	AudiencePublic  Audience = "EVERYONE"
	AudienceFriends Audience = "FRIENDS"
	AudienceOnlyMe  Audience = "SELF"
)