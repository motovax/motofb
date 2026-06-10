package models

// Attachment is a union of attachment payloads.
type Attachment struct {
	Type AttachmentType
	Data any
}

type ImageAttachment struct {
	ID       string
	Filename string
	Preview  string
	Width    int
	Height   int
}

type VideoAttachment struct {
	ID              string
	PlayableURL     string
	Preview         string
	PlayableDuration int
}

type StickerAttachment struct {
	ID  string
	URL string
}

type FileAttachment struct {
	ID          string
	DownloadURL string
	Filename    string
}

type AudioAttachment struct {
	ID          string
	DownloadURL string
}

type LocationAttachment struct {
	ID        string
	URL       string
	Latitude  float64
	Longitude float64
	Address   string
	IsLive    bool
}

type PostAttachment struct {
	ID          string
	Title       string
	Description string
	PostURL     string
}

type ProfileAttachment struct {
	ID             string
	ProfileID      string
	ProfileName    string
	ProfileURL     string
	ProfilePicture string
	CoverPhoto     string
}

type ExternalAttachment struct {
	ID          string
	URL         string
	Title       string
	Description string
}

type ReelAttachment struct {
	ID          string
	URL         string
	VideoID     string
	Title       string
	Description string
}

type ProductAttachment struct {
	ID           string
	ProductName  string
	ProductPrice string
	URL          string
}

type SharedAttachment struct {
	ID          string
	Title       string
	Description string
}