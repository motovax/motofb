package internal_test

import (
	"testing"

	"github.com/motovax/motofb/internal"
)

func TestDecimalToBase36(t *testing.T) {
	tests := map[int]string{
		0:  "0",
		1:  "1",
		35: "z",
		36: "10",
	}
	for in, want := range tests {
		if got := internal.DecimalToBase36(in); got != want {
			t.Fatalf("DecimalToBase36(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestJazoest(t *testing.T) {
	got := internal.Jazoest("abc")
	want := "2294" // 2 + ord('a')+ord('b')+ord('c')
	if got != want {
		t.Fatalf("Jazoest = %q, want %q", got, want)
	}
}

func TestGenerateOfflineThreadingID(t *testing.T) {
	id := internal.GenerateOfflineThreadingID()
	if id == "" {
		t.Fatal("expected non-empty offline threading id")
	}
}