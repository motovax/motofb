// Package internal holds helpers ported from fbchat-muqit utils.
package internal

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
)

// NowMillis returns the current Unix timestamp in milliseconds.
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// GenerateUUID returns a random UUID v4 string.
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateClientID returns a hex client id similar to Python client_id_factory.
func GenerateClientID() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<31))
	if err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", n.Int64())
}

// GenerateMessageID builds a Mercury-style message id.
func GenerateMessageID(clientID string) string {
	k := NowMillis()
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	l := binary.BigEndian.Uint32(buf[:])
	return fmt.Sprintf("<%d:%d-%s@mail.projektitan.com>", k, l, clientID)
}

// GenerateOfflineThreadingID builds an offline threading id for /ls_req payloads.
func GenerateOfflineThreadingID() string {
	ret := NowMillis()
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	value := binary.BigEndian.Uint32(buf[:])
	bin := fmt.Sprintf("%022b", value)
	msgs := fmt.Sprintf("%b%s", ret, bin[len(bin)-22:])
	n := new(big.Int)
	n.SetString(msgs, 2)
	return n.String()
}

// DecimalToBase36 converts a decimal counter to Facebook's __req format.
func DecimalToBase36(number int) string {
	if number == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := ""
	n := number
	for n > 0 {
		rem := n % 36
		n /= 36
		result = string(digits[rem]) + result
	}
	return result
}

// Jazoest computes jazoest from fb_dtsg token characters.
func Jazoest(token string) string {
	sum := 0
	for _, c := range token {
		sum += int(c)
	}
	return fmt.Sprintf("2%d", sum)
}

// PrefixURL adds protocol and host when url is relative.
func PrefixURL(url, host string) string {
	if len(url) > 0 && url[0] == '/' {
		return "https://" + host + url
	}
	return url
}

// MIMEToKey maps MIME types to Facebook attachment id keys.
func MIMEToKey(mime string) string {
	if mime == "" {
		return "file_id"
	}
	if mime == "image/gif" {
		return "gif_id"
	}
	switch {
	case len(mime) > 6 && mime[:6] == "video/":
		return "video_id"
	case len(mime) > 6 && mime[:6] == "image/":
		return "image_id"
	case len(mime) > 6 && mime[:6] == "audio/":
		return "audio_id"
	default:
		return "file_id"
	}
}