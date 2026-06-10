package state

import fberr "github.com/motovax/motofb/errors"

// ErrNotLoggedIn reports an uninitialized or expired session.
func ErrNotLoggedIn() error {
	return fberr.Wrap("state", "client is not logged in", fberr.ErrNotLoggedIn)
}