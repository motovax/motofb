package messenger

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/motovax/motofb/internal"
)

// Version and app IDs from captured Facebook payloads.
const (
	AppMessengerPrimary = "2220391788200892"
	AppMessengerLS      = "772021112871879"

	VerSendMessage      = "6120284488008082"
	VerLSDefault        = "9507618899363250"
	VerForwardSearch    = "24628521740133582"
	VerUnsendUnread     = "24959613840289226"
	VerReactCreateGroup = "25137701082502211"
	VerMarkRead         = "24279165305039531"
	VerThreadSettings   = "31712138825101068"
	VerTyping           = "5849951561777440"
)

// LSRequester publishes /ls_req payloads and correlates /ls_resp.
type LSRequester struct {
	mu       sync.Mutex
	taskID   int
	reqID    int
	publish  func(topic string, payload []byte) error
	pending  map[int]chan LSResponse
}

// LSResponse is a /ls_resp message.
type LSResponse struct {
	RequestID int
	Payload   string
}

func NewLSRequester(publish func(topic string, payload []byte) error) *LSRequester {
	return &LSRequester{
		publish: publish,
		pending: make(map[int]chan LSResponse),
	}
}

func (r *LSRequester) nextTaskID() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.taskID
	r.taskID++
	return id
}

func (r *LSRequester) nextRequestID() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.reqID
	r.reqID++
	return id
}

// Resolve completes a pending request from /ls_resp.
func (r *LSRequester) Resolve(resp LSResponse) {
	r.mu.Lock()
	ch := r.pending[resp.RequestID]
	delete(r.pending, resp.RequestID)
	r.mu.Unlock()
	if ch != nil {
		ch <- resp
	}
}

type lsTask struct {
	Label        string         `json:"label"`
	Payload      string         `json:"payload"`
	QueueName    string         `json:"queue_name"`
	TaskID       int            `json:"task_id"`
	FailureCount *int           `json:"failure_count"`
}

type lsInner struct {
	EpochID     int64    `json:"epoch_id"`
	Tasks       []lsTask `json:"tasks"`
	VersionID   string   `json:"version_id"`
	DataTraceID *string  `json:"data_trace_id"`
}

type lsEnvelope struct {
	AppID     string `json:"app_id"`
	Payload   string `json:"payload"`
	RequestID int    `json:"request_id"`
	Type      int    `json:"type"`
}

// Task describes one ls_req task before JSON stringification.
type Task struct {
	Label     string
	Payload   map[string]any
	QueueName string
}

// PublishTasks sends a multi-task /ls_req without waiting for response.
func (r *LSRequester) PublishTasks(appID, versionID string, tasks []Task) error {
	env, err := r.buildEnvelope(appID, versionID, tasks, 3)
	if err != nil {
		return err
	}
	return r.publish("/ls_req", env)
}

// SendTasks publishes and waits for matching /ls_resp.
func (r *LSRequester) SendTasks(appID, versionID string, tasks []Task) (LSResponse, error) {
	env, reqID, err := r.buildEnvelopeWithID(appID, versionID, tasks, 3)
	if err != nil {
		return LSResponse{}, err
	}
	ch := make(chan LSResponse, 1)
	r.mu.Lock()
	r.pending[reqID] = ch
	r.mu.Unlock()

	if err := r.publish("/ls_req", env); err != nil {
		r.mu.Lock()
		delete(r.pending, reqID)
		r.mu.Unlock()
		return LSResponse{}, err
	}
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(10 * time.Second):
		r.mu.Lock()
		delete(r.pending, reqID)
		r.mu.Unlock()
		return LSResponse{}, internal.ErrTimeout("ls_req response timed out")
	}
}

// PublishTyping sends type-4 typing envelope.
func (r *LSRequester) PublishTyping(uid string, threadID string, isGroup, isTyping bool, threadType int) error {
	inner := map[string]any{
		"label": "3",
		"payload": mustJSON(map[string]any{
			"thread_key":      atoi(threadID),
			"is_group_thread": bool01(isGroup),
			"is_typing":       bool01(isTyping),
			"attribution":     0,
			"sync_group":      1,
			"thread_type":     threadType,
		}),
		"version": VerTyping,
	}
	env := map[string]any{
		"app_id":     AppMessengerPrimary,
		"payload":    mustJSON(inner),
		"request_id": r.nextRequestID(),
		"type":       4,
	}
	b, _ := json.Marshal(env)
	return r.publish("/ls_req", b)
}

func (r *LSRequester) buildEnvelope(appID, versionID string, tasks []Task, typ int) ([]byte, error) {
	b, _, err := r.buildEnvelopeWithID(appID, versionID, tasks, typ)
	return b, err
}

func (r *LSRequester) buildEnvelopeWithID(appID, versionID string, tasks []Task, typ int) ([]byte, int, error) {
	lsTasks := make([]lsTask, 0, len(tasks))
	for _, t := range tasks {
		lsTasks = append(lsTasks, lsTask{
			Label:        t.Label,
			Payload:      mustJSON(t.Payload),
			QueueName:    t.QueueName,
			TaskID:       r.nextTaskID(),
			FailureCount: nil,
		})
	}
	inner := lsInner{
		EpochID:     atoi64(internal.GenerateOfflineThreadingID()),
		Tasks:       lsTasks,
		VersionID:   versionID,
		DataTraceID: nil,
	}
	reqID := r.nextRequestID()
	env := lsEnvelope{
		AppID:     appID,
		Payload:   mustJSON(inner),
		RequestID: reqID,
		Type:      typ,
	}
	b, err := json.Marshal(env)
	return b, reqID, err
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func bool01(v bool) int {
	if v {
		return 1
	}
	return 0
}

// ExtractMessageID parses message id from /ls_resp payload.
func ExtractMessageID(payload string) string {
	marker := "replaceOptimsiticMessage"
	idx := strings.Index(payload, marker)
	if idx < 0 {
		return ""
	}
	midStart := strings.Index(payload[idx:], "mid.")
	if midStart < 0 {
		return ""
	}
	midStart += idx
	midEnd := strings.Index(payload[midStart:], `"`)
	if midEnd < 0 {
		return payload[midStart:]
	}
	return payload[midStart : midStart+midEnd]
}

// ExtractThreadID parses thread id from optimistic thread response.
func ExtractThreadID(payload string) string {
	marker := "replaceOptimisticThread"
	idx := strings.Index(payload, marker)
	if idx < 0 {
		return ""
	}
	key := `"thread_key":`
	pos := strings.Index(payload[idx:], key)
	if pos < 0 {
		return ""
	}
	pos += idx + len(key)
	rest := strings.TrimSpace(payload[pos:])
	rest = strings.Trim(rest, `",`)
	return rest
}