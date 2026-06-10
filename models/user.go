package models

// User is a Facebook/Messenger user profile.
type User struct {
	ID            string
	Name          string
	FirstName     string
	Username      string
	Gender        string
	URL           string
	IsFriend      bool
	IsBlocked     bool
	Image         string
	AlternateName string
}