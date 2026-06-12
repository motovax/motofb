// Package facebook implements Facebook feed and friending GraphQL operations.
package facebook

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/graphql"
	"github.com/motovax/motofb/internal"
	"github.com/motovax/motofb/models"
	"github.com/motovax/motofb/state"
)

const (
	docFriendRequestConfirm = "24205795295769853"
	docFriendRequestDelete  = "25003074442651692"
	docFriendRequestSend    = "24974393785534352"
	docUnfriend             = "24028849793460009"
	docFriendRequestCancel  = "24453541284254355"
	docReactToPost          = "24034997962776771"
	docPrivacyPicker        = "24820345800985339"
	docFeedComposerRoot     = "32319398104317803"
	docPrivacyRefetch       = "24578653808469895"
	docComposerStoryCreate  = "24966185093062904"
)

var (
	rePrivacyWriteID  = regexp.MustCompile(`"privacy_write_id"\s*:\s*"([^"]+)"`)
	rePrivacyRowInput = regexp.MustCompile(`"privacy_row_input"\s*:\s*({[^{}]+})`)
)

// Client handles Facebook posts, reactions, and friend management.
type Client struct {
	state      *state.State
	gql        *graphql.Processor
	mutationID int
	mu         sync.Mutex

	origin  string
	referer string
}

// New constructs a Facebook API client backed by an authenticated session.
func New(st *state.State) *Client {
	origin := "https://www.facebook.com"
	referer := "https://www.facebook.com/"
	if st != nil && st.Host != "" {
		origin = "https://" + st.Host
		referer = origin + "/"
	}
	return &Client{
		state:   st,
		gql:     graphql.NewProcessor(),
		origin:  origin,
		referer: referer,
	}
}

// GetMutationID returns the next client_mutation_id for GraphQL mutations.
func (c *Client) GetMutationID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.mutationID
	c.mutationID++
	return strconv.Itoa(id)
}

func (c *Client) requireState(op string) error {
	if c.state == nil || !c.state.LoggedIn {
		return state.ErrNotLoggedIn()
	}
	_ = op
	return nil
}

// ManageFriendRequest accepts or rejects an incoming friend request.
func (c *Client) ManageFriendRequest(ctx context.Context, userID string, accept bool) error {
	if err := c.requireState("ManageFriendRequest"); err != nil {
		return err
	}

	ts := strconv.FormatInt(internal.NowMillis(), 10)
	variables := map[string]any{
		"input": map[string]any{
			"click_correlation_id":          ts,
			"click_proof_validation_result": `{"validated":true}`,
			"friend_requester_id":           userID,
			"friending_channel":             "FRIENDS_HOME_REQUESTS",
			"actor_id":                      c.state.UserID,
			"client_mutation_id":            c.GetMutationID(),
		},
		"scale":       3,
		"refresh_num": 0,
	}

	friendlyName := "FriendingCometFriendRequestDeleteMutation"
	docID := docFriendRequestDelete
	if accept {
		friendlyName = "FriendingCometFriendRequestConfirmMutation"
		docID = docFriendRequestConfirm
		if input, ok := variables["input"].(map[string]any); ok {
			input["warn_ack"] = false
		}
		variables["should_fix_banner"] = true
	}

	data, err := c.graphqlForm(friendlyName, docID, variables, false)
	if err != nil {
		return err
	}
	return c.state.PostNoResponse(ctx, "https://www.facebook.com/api/graphql/", data)
}

// SendFriendRequest sends friend requests to one or more users.
func (c *Client) SendFriendRequest(ctx context.Context, userIDs []string) error {
	if err := c.requireState("SendFriendRequest"); err != nil {
		return err
	}

	variables := map[string]any{
		"input": map[string]any{
			"click_correlation_id":          strconv.FormatInt(internal.NowMillis(), 10),
			"click_proof_validation_result": `{"validated":true}`,
			"friend_requestee_ids":          userIDs,
			"friending_channel":             "FRIENDS_HOME_MAIN",
			"warn_ack_for_ids":              []string{},
			"actor_id":                      c.state.UserID,
			"client_mutation_id":            c.GetMutationID(),
		},
		"scale": 3,
	}

	data, err := c.graphqlForm("FriendingCometFriendRequestSendMutation", docFriendRequestSend, variables, false)
	if err != nil {
		return err
	}
	return c.state.PostNoResponse(ctx, "/api/graphql/", data)
}

// Unfriend removes an existing friend.
func (c *Client) Unfriend(ctx context.Context, userID string) error {
	if err := c.requireState("Unfriend"); err != nil {
		return err
	}

	variables := map[string]any{
		"input": map[string]any{
			"source":             "bd_profile_button",
			"unfriended_user_id": userID,
			"actor_id":           c.state.UserID,
			"client_mutation_id": c.GetMutationID(),
		},
		"scale": 3,
	}

	data, err := c.graphqlForm("FriendingCometUnfriendMutation", docUnfriend, variables, false)
	if err != nil {
		return err
	}
	_, err = c.state.PostGraphQLSingle(ctx, data)
	return err
}

// CancelFriendRequest cancels a previously sent friend request.
func (c *Client) CancelFriendRequest(ctx context.Context, userID string) error {
	if err := c.requireState("CancelFriendRequest"); err != nil {
		return err
	}

	variables := map[string]any{
		"input": map[string]any{
			"cancelled_friend_requestee_id": userID,
			"click_correlation_id":          strconv.FormatInt(internal.NowMillis(), 10),
			"click_proof_validation_result": `{"validated":true}`,
			"friending_channel":             "PROFILE_BUTTON",
			"actor_id":                      c.state.UserID,
			"client_mutation_id":            c.GetMutationID(),
		},
		"scale": 3,
	}

	data, err := c.graphqlForm("FriendingCometFriendRequestCancelMutation", docFriendRequestCancel, variables, false)
	if err != nil {
		return err
	}
	_, err = c.state.PostGraphQLSingle(ctx, data)
	return err
}

// ReactToPostOptions configures a post reaction mutation.
type ReactToPostOptions struct {
	FeedbackID string
	PostID     *int64
	Reaction   models.FBReaction
}

// ReactToPost reacts to a Facebook post by feedback id or numeric post id.
func (c *Client) ReactToPost(ctx context.Context, opts ReactToPostOptions) error {
	if err := c.requireState("ReactToPost"); err != nil {
		return err
	}

	feedbackID := opts.FeedbackID
	if feedbackID == "" && opts.PostID == nil {
		return fberr.Wrap("ReactToPost", "either FeedbackID or PostID must be provided", fberr.ErrValidation)
	}
	if feedbackID == "" && opts.PostID != nil {
		feedbackID = postIDToFeedbackID(*opts.PostID)
	}

	reaction := opts.Reaction
	if reaction == "" {
		reaction = models.FBReactionLove
	}

	variables := map[string]any{
		"input": map[string]any{
			"attribution_id_v2": fmt.Sprintf(
				"CometHomeRoot.react,comet.home,tap_tabbar,%d,420553,4748854339,,",
				internal.NowMillis(),
			),
			"feedback_id":           feedbackID,
			"feedback_reaction_id":  string(reaction),
			"feedback_source":       "NEWS_FEED",
			"is_tracking_encrypted": true,
			"tracking":              []any{},
			"session_id":            internal.GenerateUUID(),
			"actor_id":              c.state.UserID,
			"client_mutation_id":    c.GetMutationID(),
		},
		"useDefaultActor": false,
		"__relay_internal__pv__CometUFIReactionsEnableShortNamerelayprovider": false,
	}

	data, err := c.graphqlForm("CometUFIFeedbackReactMutation", docReactToPost, variables, true)
	if err != nil {
		return err
	}
	return c.state.PostNoResponse(ctx, "/api/graphql/", data)
}

// PublishPostOptions configures a feed post publish mutation.
type PublishPostOptions struct {
	Text          string
	ImagePaths    []string
	TagUsers      []string
	SpecificUsers []string
	ExceptUsers   []string
	Audience      models.Audience
	Mentions      []models.Mention
}

// PublishPost publishes a text post with optional images and audience controls.
func (c *Client) PublishPost(ctx context.Context, opts PublishPostOptions) (string, error) {
	if err := c.requireState("PublishPost"); err != nil {
		return "", err
	}

	audience := opts.Audience
	if audience == "" {
		audience = models.AudiencePublic
	}

	var uploadedPhotoIDs []string
	if len(opts.ImagePaths) > 0 {
		ids, err := c.UploadPhotos(ctx, opts.ImagePaths, 5)
		if err != nil {
			return "", err
		}
		uploadedPhotoIDs = ids
	}

	audienceData, err := c.formatAudience(ctx, opts.SpecificUsers, opts.ExceptUsers, audience)
	if err != nil {
		return "", err
	}

	sessionID := internal.GenerateUUID()
	variables := map[string]any{
		"input": map[string]any{
			"composer_entry_point":    "inline_composer",
			"composer_source_surface": "newsfeed",
			"composer_type":           "feed",
			"idempotence_token":       sessionID + "_FEED",
			"source":                  "WWW",
			"audience":                audienceData,
			"message": map[string]any{
				"ranges": mentionsToRanges(opts.Mentions),
				"text":   opts.Text,
			},
			"inline_activities":     []any{},
			"text_format_preset_id": "0",
			"publishing_flow": map[string]any{
				"supported_flows": []string{"ASYNC_SILENT", "ASYNC_NOTIF", "FALLBACK"},
			},
			"attachments":   postAttachments(uploadedPhotoIDs, nil),
			"with_tags_ids": opts.TagUsers,
			"logging": map[string]any{
				"composer_session_id": sessionID,
			},
			"navigation_data": map[string]any{
				"attribution_id_v2": fmt.Sprintf(
					"CometHomeRoot.react,comet.home,via_cold_start,%d,929297,4748854339,,",
					internal.NowMillis(),
				),
			},
			"tracking": []any{nil},
			"event_share_metadata": map[string]any{
				"surface": "newsfeed",
			},
			"actor_id":           c.state.UserID,
			"client_mutation_id": "1",
		},
		"feedLocation":                        "NEWSFEED",
		"feedbackSource":                      1,
		"focusCommentID":                      nil,
		"gridMediaWidth":                      nil,
		"groupID":                             nil,
		"scale":                               3,
		"privacySelectorRenderLocation":       "COMET_STREAM",
		"checkPhotosToReelsUpsellEligibility": true,
		"renderLocation":                      "homepage_stream",
		"useDefaultActor":                     false,
		"inviteShortLinkKey":                  nil,
		"isFeed":                              true,
		"isFundraiser":                        false,
		"isFunFactPost":                       false,
		"isGroup":                             false,
		"isEvent":                             false,
		"isTimeline":                          false,
		"isSocialLearning":                    false,
		"isPageNewsFeed":                      false,
		"isProfileReviews":                    false,
		"isWorkSharedDraft":                   false,
		"hashtag":                             nil,
		"canUserManageOffers":                 false,
		"__relay_internal__pv__CometUFIShareActionMigrationrelayprovider":                     true,
		"__relay_internal__pv__GHLShouldChangeSponsoredDataFieldNamerelayprovider":            true,
		"__relay_internal__pv__GHLShouldChangeAdIdFieldNamerelayprovider":                     true,
		"__relay_internal__pv__CometUFI_dedicated_comment_routable_dialog_gkrelayprovider":    false,
		"__relay_internal__pv__CometUFICommentAvatarStickerAnimatedImagerelayprovider":        false,
		"__relay_internal__pv__IsWorkUserrelayprovider":                                       false,
		"__relay_internal__pv__CometUFIReactionsEnableShortNamerelayprovider":                 false,
		"__relay_internal__pv__FBReels_enable_view_dubbed_audio_type_gkrelayprovider":         true,
		"__relay_internal__pv__FBReels_deprecate_short_form_video_context_gkrelayprovider":    true,
		"__relay_internal__pv__FeedDeepDiveTopicPillThreadViewEnabledrelayprovider":           false,
		"__relay_internal__pv__CometImmersivePhotoCanUserDisable3DMotionrelayprovider":        false,
		"__relay_internal__pv__WorkCometIsEmployeeGKProviderrelayprovider":                    false,
		"__relay_internal__pv__IsMergQAPollsrelayprovider":                                    false,
		"__relay_internal__pv__FBReels_enable_meta_ai_label_gkrelayprovider":                  true,
		"__relay_internal__pv__FBReelsMediaFooter_comet_enable_reels_ads_gkrelayprovider":     true,
		"__relay_internal__pv__StoriesArmadilloReplyEnabledrelayprovider":                     true,
		"__relay_internal__pv__FBReelsIFUTileContent_reelsIFUPlayOnHoverrelayprovider":        true,
		"__relay_internal__pv__GroupsCometGYSJFeedItemHeightrelayprovider":                    150,
		"__relay_internal__pv__StoriesShouldIncludeFbNotesrelayprovider":                      false,
		"__relay_internal__pv__GHLShouldChangeSponsoredAuctionDistanceFieldNamerelayprovider": false,
		"__relay_internal__pv__GHLShouldUseSponsoredAuctionLabelFieldNameV1relayprovider":     false,
		"__relay_internal__pv__GHLShouldUseSponsoredAuctionLabelFieldNameV2relayprovider":     false,
	}

	data, err := c.graphqlForm("ComposerStoryCreateMutation", docComposerStoryCreate, variables, true)
	if err != nil {
		return "", err
	}

	extra := http.Header{
		"Sec-Fetch-Mode": {"cors"},
		"Sec-Fetch-Site": {"same-origin"},
		"Origin":         {c.origin},
		"Referer":        {c.referer},
	}
	raw, err := c.postGraphQLWithHeaders(ctx, data, extra)
	if err != nil {
		return "", err
	}

	return parsePublishPostResponse(raw)
}

// UploadPhoto uploads a single image for composer attachments.
func (c *Client) UploadPhoto(ctx context.Context, imagePath string) (string, error) {
	if err := c.requireState("UploadPhoto"); err != nil {
		return "", err
	}

	filePath := filepath.Clean(imagePath)
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return "", fberr.Wrap("UploadPhoto", fmt.Sprintf("file not found: %s", imagePath), fberr.ErrValidation)
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fberr.Wrap("UploadPhoto", "read file", err)
	}

	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = http.DetectContentType(fileData)
	}

	uploadID := "jsc_c_" + internal.GenerateUUID()[:8]
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fields := map[string]string{
		"lsd":           c.state.LSD,
		"source":        "8",
		"profile_id":    c.state.UserID,
		"waterfallxapp": "comet",
		"upload_id":     uploadID,
	}
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return "", err
		}
	}

	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="farr"; filename="%s"`, filepath.Base(filePath)),
	}
	partHeader["Content-Type"] = []string{contentType}
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(fileData); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	url := "https://upload.facebook.com/ajax/react_composer/attachments/photo/upload"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", err
	}
	for k, vals := range c.state.BuildHeaders(url, "upload", "") {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")

	resp, err := c.state.HTTP.Do(req)
	if err != nil {
		return "", fberr.Wrap("UploadPhoto", "http post", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fberr.Wrap("UploadPhoto", fmt.Sprintf("status %d", resp.StatusCode), fberr.ErrNetwork)
	}

	return parsePhotoUploadResponse(raw)
}

// UploadPhotos uploads multiple images concurrently and returns successful photo ids in order.
func (c *Client) UploadPhotos(ctx context.Context, imagePaths []string, maxConcurrent int) ([]string, error) {
	if len(imagePaths) == 0 {
		return nil, nil
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	type result struct {
		index int
		id    string
		err   error
	}

	sem := make(chan struct{}, maxConcurrent)
	results := make(chan result, len(imagePaths))
	var wg sync.WaitGroup

	for i, path := range imagePaths {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			id, err := c.UploadPhoto(ctx, p)
			results <- result{index: idx, id: id, err: err}
		}(i, path)
	}

	wg.Wait()
	close(results)

	ordered := make([]string, len(imagePaths))
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		ordered[r.index] = r.id
	}

	photoIDs := make([]string, 0, len(imagePaths))
	for _, id := range ordered {
		if id != "" {
			photoIDs = append(photoIDs, id)
		}
	}

	if len(photoIDs) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return photoIDs, nil
}

type privacyRow struct {
	ID              string
	PrivacyRowInput privacyValues
}

type privacyValues struct {
	Allow     []any           `json:"allow"`
	Deny      []any           `json:"deny"`
	BaseState models.Audience `json:"base_state"`
}

func (c *Client) graphqlForm(friendlyName, docID string, variables any, includeLSD bool) (map[string]string, error) {
	vb, err := json.Marshal(variables)
	if err != nil {
		return nil, fberr.Wrap("graphqlForm", "marshal variables", err)
	}
	data := map[string]string{
		"fb_api_caller_class":      "RelayModern",
		"fb_api_req_friendly_name": friendlyName,
		"server_timestamps":        "true",
		"doc_id":                   docID,
		"variables":                string(vb),
	}
	if includeLSD && c.state.LSD != "" {
		data["lsd"] = c.state.LSD
	}
	return data, nil
}

func (c *Client) postGraphQLWithHeaders(ctx context.Context, data map[string]string, extra http.Header) ([]byte, error) {
	fullURL := internal.PrefixURL("/api/graphql/", c.state.Host)
	params := c.state.NextReqParams()
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	for k, v := range data {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	friendly := data["fb_api_req_friendly_name"]
	for k, vals := range c.state.BuildHeaders(fullURL, "post", friendly) {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	for k, vals := range extra {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.state.HTTP.Do(req)
	if err != nil {
		return nil, fberr.Wrap("postGraphQLWithHeaders", "http post", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fberr.Wrap("postGraphQLWithHeaders", fmt.Sprintf("status %d", resp.StatusCode), fberr.ErrNetwork)
	}
	return body, nil
}

func (c *Client) getPrivacyWriter(ctx context.Context) (*privacyRow, error) {
	variables := map[string]any{
		"hasStory":                      false,
		"isBizWeb":                      false,
		"privacySelectorRenderLocation": "COMET_COMPOSER",
		"profileID":                     c.state.UserID,
		"scale":                         3,
		"storyID":                       "",
		"__relay_internal__pv__FeedComposerComet_isGenAILabelEnabledrelayprovider":                     false,
		"__relay_internal__pv__CometUnifiedVideoCreation_showPrivacyMergereLayprovider":                false,
		"__relay_internal__pv__CometUGCPublicCreation_showComposerPublicAwarenessTooltiprelayprovider": false,
	}

	data, err := c.graphqlForm("FeedComposerCometRootQuery", docFeedComposerRoot, variables, false)
	if err != nil {
		return nil, err
	}

	raw, err := c.state.PostGraphQLSingle(ctx, data)
	if err != nil {
		return nil, err
	}

	row, err := extractPrivacyData(string(raw))
	if err != nil {
		return nil, fberr.Wrap("getPrivacyWriter", "parse privacy writer", fberr.ErrParsing)
	}
	return row, nil
}

func (c *Client) pickContainerQuery(ctx context.Context, privacyWriterID string, privacy map[string]any) error {
	variables := map[string]any{
		"localPrivacyRow": privacy,
		"privacyWriteID":  privacyWriterID,
		"renderLocation":  "COMET_COMPOSER",
		"scale":           3,
	}

	data, err := c.graphqlForm("CometPrivacySelectorPickerContainerQuery", docPrivacyPicker, variables, false)
	if err != nil {
		return err
	}
	return c.state.PostNoResponse(ctx, "/api/graphql/", data)
}

func (c *Client) setPrivacyWriter(
	ctx context.Context,
	ids []string,
	writer *privacyRow,
	allow, deny bool,
	baseStateOverride models.Audience,
) (*privacyValues, error) {
	baseState := writer.PrivacyRowInput.BaseState
	if baseState == "" {
		baseState = models.AudienceFriends
	}
	if baseStateOverride != "" {
		baseState = baseStateOverride
	}

	variables := map[string]any{
		"localPrivacyRow": map[string]any{
			"allow":               condIDs(ids, allow),
			"base_state":          string(baseState),
			"deny":                condIDs(ids, deny),
			"tag_expansion_state": "UNSPECIFIED",
		},
		"privacyWriteID": writer.ID,
		"renderLocation": "COMET_COMPOSER",
		"scale":          3,
		"tags":           nil,
	}

	data, err := c.graphqlForm("refetchCometPrivacySelectorNonAutosavePickerQuery", docPrivacyRefetch, variables, true)
	if err != nil {
		return nil, err
	}

	raw, err := c.state.PostGraphQLSingle(ctx, data)
	if err != nil {
		return nil, err
	}
	if baseStateOverride != "" {
		return nil, nil
	}

	cleaned, err := c.gql.StripJSONCruft(string(raw))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Node struct {
				Scope struct {
					SelectedRowOverride privacyValues `json:"selected_row_override"`
				} `json:"scope"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fberr.Wrap("setPrivacyWriter", "unmarshal response", err)
	}
	override := resp.Data.Node.Scope.SelectedRowOverride
	return &override, nil
}

func (c *Client) formatAudience(
	ctx context.Context,
	specificUsers, exceptUsers []string,
	baseState models.Audience,
) (map[string]any, error) {
	if len(specificUsers) > 0 && len(exceptUsers) > 0 {
		return nil, fberr.Wrap("formatAudience", "provide specific_users or except_users, not both", fberr.ErrFacebookAPI)
	}

	writer, err := c.getPrivacyWriter(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.pickContainerQuery(ctx, writer.ID, nil); err != nil {
		return nil, err
	}

	currentBase := writer.PrivacyRowInput.BaseState
	if currentBase == "" {
		currentBase = models.AudienceFriends
	}

	allow := append([]string(nil), specificUsers...)
	deny := append([]string(nil), exceptUsers...)

	if string(baseState) != string(currentBase) && len(specificUsers) == 0 && len(exceptUsers) == 0 {
		if _, err := c.setPrivacyWriter(ctx, nil, writer, false, false, baseState); err != nil {
			return nil, err
		}
	}

	if len(specificUsers) > 0 {
		override, err := c.setPrivacyWriter(ctx, specificUsers, writer, true, false, "")
		if err != nil {
			return nil, err
		}
		if override != nil {
			allow = stringifyAnySlice(override.Allow)
			deny = stringifyAnySlice(override.Deny)
			if override.BaseState != "" {
				baseState = override.BaseState
			}
		}
	} else if len(exceptUsers) > 0 {
		override, err := c.setPrivacyWriter(ctx, exceptUsers, writer, false, true, "")
		if err != nil {
			return nil, err
		}
		if override != nil {
			allow = stringifyAnySlice(override.Allow)
			deny = stringifyAnySlice(override.Deny)
			if override.BaseState != "" {
				baseState = override.BaseState
			}
		}
	}

	return map[string]any{
		"privacy": map[string]any{
			"allow":               allow,
			"base_state":          string(baseState),
			"deny":                deny,
			"tag_expansion_state": "UNSPECIFIED",
		},
	}, nil
}

func extractPrivacyData(resp string) (*privacyRow, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, resp)

	m := rePrivacyWriteID.FindStringSubmatch(cleaned)
	if len(m) < 2 {
		return nil, fberr.New("extractPrivacyData", "privacy_write_id not found")
	}

	row := &privacyRow{ID: m[1]}
	if m := rePrivacyRowInput.FindStringSubmatch(cleaned); len(m) >= 2 {
		var input privacyValues
		if err := json.Unmarshal([]byte(m[1]), &input); err == nil {
			row.PrivacyRowInput = input
		}
	}
	return row, nil
}

func parsePhotoUploadResponse(raw []byte) (string, error) {
	start := bytes.IndexByte(raw, '{')
	if start < 0 {
		return "", fberr.Wrap("parsePhotoUploadResponse", "json object not found", fberr.ErrParsing)
	}

	cleaned, err := graphql.NewProcessor().StripJSONCruft(string(raw[start:]))
	if err != nil {
		return "", err
	}

	var resp struct {
		Payload struct {
			PhotoID string `json:"photoID"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return "", fberr.Wrap("parsePhotoUploadResponse", "unmarshal response", err)
	}
	if resp.Payload.PhotoID == "" {
		return "", fberr.New("parsePhotoUploadResponse", "photoID missing from upload response")
	}
	return resp.Payload.PhotoID, nil
}

func parsePublishPostResponse(raw []byte) (string, error) {
	cleaned, err := graphql.NewProcessor().StripJSONCruft(string(raw))
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			StoryCreate struct {
				StoryID *string `json:"story_id"`
				Story   *struct {
					ID string `json:"id"`
				} `json:"story"`
				PostID any `json:"post_id"`
			} `json:"story_create"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return "", fberr.Wrap("parsePublishPostResponse", "unmarshal response", fberr.ErrParsing)
	}

	create := resp.Data.StoryCreate
	if create.StoryID != nil && *create.StoryID != "" {
		return *create.StoryID, nil
	}
	if create.Story != nil && create.Story.ID != "" {
		return create.Story.ID, nil
	}
	if create.PostID != nil {
		return fmt.Sprint(create.PostID), nil
	}
	return "", fberr.New("parsePublishPostResponse", "could not get story_id or post_id")
}

func mentionsToRanges(mentions []models.Mention) []map[string]any {
	if len(mentions) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(mentions))
	for _, m := range mentions {
		out = append(out, map[string]any{
			"entity": map[string]any{"id": m.UserID},
			"length": m.Length,
			"offset": m.Offset,
		})
	}
	return out
}

func postAttachments(pictureIDs, videoIDs []string) []map[string]any {
	switch {
	case len(pictureIDs) > 0 && len(videoIDs) > 0:
		out := make([]map[string]any, 0, len(pictureIDs)+len(videoIDs))
		for _, id := range pictureIDs {
			out = append(out, map[string]any{"photo": map[string]any{"id": id}})
		}
		for _, id := range videoIDs {
			out = append(out, map[string]any{
				"video": map[string]any{
					"audio_descriptions":                 nil,
					"id":                                 id,
					"notify_when_processed":              true,
					"transcriptions":                     nil,
					"was_created_via_unified_video_flow": nil,
				},
			})
		}
		return out
	case len(pictureIDs) > 0:
		out := make([]map[string]any, 0, len(pictureIDs))
		for _, id := range pictureIDs {
			out = append(out, map[string]any{"photo": map[string]any{"id": id}})
		}
		return out
	case len(videoIDs) > 0:
		out := make([]map[string]any, 0, len(videoIDs))
		for _, id := range videoIDs {
			out = append(out, map[string]any{
				"video": map[string]any{
					"audio_descriptions":                 nil,
					"id":                                 id,
					"notify_when_processed":              true,
					"transcriptions":                     nil,
					"was_created_via_unified_video_flow": nil,
				},
			})
		}
		return out
	default:
		return []map[string]any{}
	}
}

func postIDToFeedbackID(postID int64) string {
	if postID == 0 {
		return base64.StdEncoding.EncodeToString(nil)
	}
	n := (bits.Len64(uint64(postID)) + 7) / 8
	buf := make([]byte, 8)
	for i := range buf {
		buf[i] = byte(postID >> (56 - 8*i))
	}
	return base64.StdEncoding.EncodeToString(buf[8-n:])
}

func condIDs(ids []string, enabled bool) []string {
	if enabled {
		return ids
	}
	return []string{}
}

func stringifyAnySlice(values []any) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, fmt.Sprint(v))
	}
	return out
}
