package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		numWords int
	}{
		{"33 bytes (24 words)", 33, 24},
		{"32 bytes (24 words)", 32, 24}, // 256 bits → ceil(256/11) = 24 words
		{"45 bytes (33 words)", 45, 33}, // 360 bits → ceil(360/11) = 33 words
		{"1 byte (1 word)", 1, 1},       // 8 bits → ceil(8/11) = 1 word
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.size)
			for i := range data {
				data[i] = byte(i * 7) // deterministic pattern
			}

			words := EncodeWords(data)
			if len(words) != tt.numWords {
				t.Fatalf("expected %d words, got %d", tt.numWords, len(words))
			}

			decoded, err := DecodeWords(words)
			if err != nil {
				t.Fatalf("DecodeWords error: %v", err)
			}

			// Decoded length is totalBits/8, which may truncate trailing padding bits
			expectedLen := (len(words) * 11) / 8
			if len(decoded) != expectedLen {
				t.Fatalf("decoded length: got %d, want %d", len(decoded), expectedLen)
			}

			// The original data should match the decoded data up to the original length
			if !bytes.Equal(decoded[:tt.size], data) {
				t.Errorf("round-trip mismatch:\n  got:  %x\n  want: %x", decoded[:tt.size], data)
			}
		})
	}
}

func TestEncodeWords24(t *testing.T) {
	// 33 bytes = 264 bits = exactly 24 words (no padding needed)
	data := make([]byte, 33)
	for i := range data {
		data[i] = byte(i + 1)
	}
	words := EncodeWords(data)
	if len(words) != 24 {
		t.Errorf("expected 24 words for 33 bytes, got %d", len(words))
	}
}

func TestDecodeWordsInvalidWord(t *testing.T) {
	words := []string{"abandon", "ability", "appler"} // "appler" is a typo for "apple"
	_, err := DecodeWords(words)
	if err == nil {
		t.Fatal("expected error for invalid word")
	}
	if !strings.Contains(err.Error(), "appler") {
		t.Errorf("error should mention the invalid word, got: %v", err)
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error should include a suggestion, got: %v", err)
	}
}

func TestDecodeWordsEmpty(t *testing.T) {
	_, err := DecodeWords([]string{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "no words") {
		t.Errorf("expected 'no words' error, got: %v", err)
	}
}

func TestSuggestWord(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"appla", "apple"},    // one char off from "apple"
		{"abandn", "abandon"}, // missing 'o'
		{"zooo", "zoo"},       // one extra char
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SuggestWord(tt.input)
			if got != tt.expected {
				t.Errorf("SuggestWord(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBIP39ListIntegrity(t *testing.T) {
	// Check count
	if len(bip39English) != 2048 {
		t.Fatalf("expected 2048 words, got %d", len(bip39English))
	}

	// Check no duplicates
	seen := make(map[string]bool, 2048)
	for i, w := range bip39English {
		if w == "" {
			t.Errorf("empty word at index %d", i)
		}
		if seen[w] {
			t.Errorf("duplicate word %q at index %d", w, i)
		}
		seen[w] = true
	}

	// SHA-256 integrity check: hash the newline-joined word list
	joined := strings.Join(bip39English[:], "\n") + "\n"
	hash := sha256.Sum256([]byte(joined))
	hexHash := hex.EncodeToString(hash[:])
	expectedHash := "2f5eed53a4727b4bf8880d8f3f199efc90e58503646d9ff8eff3a2ed3b24dbda"
	if hexHash != expectedHash {
		t.Errorf("BIP39 word list hash mismatch:\n  got:  %s\n  want: %s", hexHash, expectedHash)
	}
}

func TestEncodeWordsDeterministic(t *testing.T) {
	data := make([]byte, 33)
	for i := range data {
		data[i] = byte(i * 13)
	}

	words1 := EncodeWords(data)
	words2 := EncodeWords(data)

	if strings.Join(words1, " ") != strings.Join(words2, " ") {
		t.Error("EncodeWords is not deterministic")
	}
}

func TestShareWords(t *testing.T) {
	data := make([]byte, 33)
	for i := range data {
		data[i] = byte(i)
	}
	share := NewShare(2, 1, 5, 3, "Alice", data)
	words := share.Words()
	if len(words) != 25 {
		t.Errorf("expected 25 words for 33-byte share (24 data + 1 index), got %d", len(words))
	}

	// Round-trip through DecodeShareWords
	decoded, index, err := DecodeShareWords(words)
	if err != nil {
		t.Fatalf("DecodeShareWords error: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Errorf("Share.Words() round-trip data mismatch")
	}
	if index != 1 {
		t.Errorf("Share.Words() round-trip index: got %d, want 1", index)
	}
}

func TestDecodeShareWordsRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		index int
	}{
		{"index 1", 1},
		{"index 2", 2},
		{"index 5", 5},
		{"index 100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, 33)
			for i := range data {
				data[i] = byte(i * 7)
			}
			share := NewShare(2, tt.index, 5, 3, "Test", data)
			words := share.Words()
			if len(words) != 25 {
				t.Fatalf("expected 25 words, got %d", len(words))
			}

			decoded, index, err := DecodeShareWords(words)
			if err != nil {
				t.Fatalf("DecodeShareWords error: %v", err)
			}
			if !bytes.Equal(decoded, data) {
				t.Errorf("data mismatch")
			}
			if index != tt.index {
				t.Errorf("index: got %d, want %d", index, tt.index)
			}
		})
	}
}
