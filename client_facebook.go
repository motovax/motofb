package motofb

import (
	"context"

	"github.com/motovax/motofb/facebook"
	"github.com/motovax/motofb/models"
)

// ManageFriendRequest accepts or rejects a friend request.
func (c *Client) ManageFriendRequest(ctx context.Context, userID string, accept bool) error {
	return c.facebook.ManageFriendRequest(ctx, userID, accept)
}

// SendFriendRequest sends friend requests to users.
func (c *Client) SendFriendRequest(ctx context.Context, userIDs []string) error {
	return c.facebook.SendFriendRequest(ctx, userIDs)
}

// Unfriend removes a friend.
func (c *Client) Unfriend(ctx context.Context, userID string) error {
	return c.facebook.Unfriend(ctx, userID)
}

// CancelFriendRequest cancels an outgoing friend request.
func (c *Client) CancelFriendRequest(ctx context.Context, userID string) error {
	return c.facebook.CancelFriendRequest(ctx, userID)
}

// ReactToPost reacts to a Facebook post.
func (c *Client) ReactToPost(ctx context.Context, opts facebook.ReactToPostOptions) error {
	return c.facebook.ReactToPost(ctx, opts)
}

// PublishPost creates a Facebook feed post.
func (c *Client) PublishPost(ctx context.Context, opts facebook.PublishPostOptions) (string, error) {
	return c.facebook.PublishPost(ctx, opts)
}

// UploadPhoto uploads one photo for composer.
func (c *Client) UploadPhoto(ctx context.Context, path string) (string, error) {
	return c.facebook.UploadPhoto(ctx, path)
}

// UploadPhotos uploads multiple photos concurrently.
func (c *Client) UploadPhotos(ctx context.Context, paths []string, maxConcurrent int) ([]string, error) {
	return c.facebook.UploadPhotos(ctx, paths, maxConcurrent)
}

// ReactToPostSimple reacts using reaction enum and post id.
func (c *Client) ReactToPostSimple(ctx context.Context, postID int64, reaction models.FBReaction) error {
	return c.facebook.ReactToPost(ctx, facebook.ReactToPostOptions{PostID: &postID, Reaction: reaction})
}