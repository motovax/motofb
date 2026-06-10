package mqtt

// Subscribed topics embedded in MQTT username (edge-chat.facebook.com).
var SubscribeTopics = []string{
	"/legacy_web",
	"/ls_req",
	"/ls_resp",
	"/t_ms",
	"/rtc_multi",
	"/thread_typing",
	"/orca_typing_notifications",
	"/orca_presence",
	"/br_sr",
	"/friend_request",
	"/friending_state_change",
	"/friend_requests_seen",
	"/sr_res",
	"/webrtc",
	"/onevc",
	"/notify_disconnect",
	"/mercury",
	"/inbox",
	"/messaging_events",
	"/orca_message_notifications",
	"/pp",
	"/webrtc_response",
}

const Host = "edge-chat.facebook.com"