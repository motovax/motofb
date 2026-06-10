package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/motovax/motofb/state"
)

func TestExtractTokensFromHTML(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "testdata", "sample_login.html"))
	if err != nil {
		t.Fatal(err)
	}

	tokens, err := state.ExtractTokensFromHTML(string(html))
	if err != nil {
		t.Fatal(err)
	}

	if tokens.FBDtsg != "NAcTestDtsgToken123" {
		t.Fatalf("FBDtsg = %q", tokens.FBDtsg)
	}
	if tokens.FBDtsgAsync != "NAcAsyncToken456" {
		t.Fatalf("FBDtsgAsync = %q", tokens.FBDtsgAsync)
	}
	if tokens.LSD != "LSD-abc123" {
		t.Fatalf("LSD = %q", tokens.LSD)
	}
	if tokens.MQTTClientID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Fatalf("MQTTClientID = %q", tokens.MQTTClientID)
	}
	if tokens.Region != "ash" {
		t.Fatalf("Region = %q", tokens.Region)
	}
	if tokens.UserName != "Test User" {
		t.Fatalf("UserName = %q", tokens.UserName)
	}
}