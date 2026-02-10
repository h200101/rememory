package core

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
)

// wordIndex maps BIP39 words to their index (0-2047). Initialized once via sync.Once.
var (
	wordIndex     map[string]int
	wordIndexOnce sync.Once
)

func initWordIndex() {
	wordIndexOnce.Do(func() {
		wordIndex = make(map[string]int, len(bip39English))
		for i, w := range bip39English {
			wordIndex[w] = i
		}
	})
}

// EncodeWords converts bytes to BIP39 words (11 bits per word).
// 33 bytes (264 bits) produces exactly 24 words.
func EncodeWords(data []byte) []string {
	totalBits := len(data) * 8
	numWords := (totalBits + 10) / 11 // ceiling division

	words := make([]string, numWords)
	for i := 0; i < numWords; i++ {
		idx := extract11Bits(data, i*11)
		words[i] = bip39English[idx]
	}
	return words
}

// extract11Bits extracts an 11-bit value starting at the given bit offset.
// Out-of-range bits are treated as zero (for padding the final chunk).
func extract11Bits(data []byte, bitOffset int) int {
	val := 0
	for b := 0; b < 11; b++ {
		byteIdx := (bitOffset + b) / 8
		bitIdx := 7 - (bitOffset+b)%8
		if byteIdx < len(data) {
			val = (val << 1) | ((int(data[byteIdx]) >> bitIdx) & 1)
		} else {
			val <<= 1 // pad with zero
		}
	}
	return val
}

// DecodeWords converts BIP39 words back to bytes.
// Returns an error with typo suggestions if a word is not recognized.
func DecodeWords(words []string) ([]byte, error) {
	initWordIndex()

	if len(words) == 0 {
		return nil, fmt.Errorf("no words provided")
	}

	// Convert words to 11-bit indices
	indices := make([]int, len(words))
	for i, w := range words {
		idx, ok := wordIndex[w]
		if !ok {
			suggestion := SuggestWord(w)
			if suggestion != "" {
				return nil, fmt.Errorf("word %d %q not recognized — did you mean %q?", i+1, w, suggestion)
			}
			return nil, fmt.Errorf("word %d %q not recognized", i+1, w)
		}
		indices[i] = idx
	}

	// Convert 11-bit indices to bytes
	totalBits := len(words) * 11
	numBytes := totalBits / 8
	result := make([]byte, numBytes)

	for i, idx := range indices {
		set11Bits(result, i*11, idx)
	}

	return result, nil
}

// set11Bits writes an 11-bit value at the given bit offset in data.
func set11Bits(data []byte, bitOffset int, val int) {
	for b := 0; b < 11; b++ {
		byteIdx := (bitOffset + b) / 8
		bitIdx := 7 - (bitOffset+b)%8
		if byteIdx < len(data) {
			if (val>>(10-b))&1 == 1 {
				data[byteIdx] |= 1 << bitIdx
			}
		}
	}
}

// Word 25 layout (11 bits total):
//
//   ┌─────────────┬──────────────────────┐
//   │ index (4 hi) │   checksum (7 lo)    │
//   │  bits 10-7   │     bits 6-0         │
//   └─────────────┴──────────────────────┘
//
// Index: share index (1-based) stored in upper 4 bits.
//   - Shares 1–15: index stored directly.
//   - Shares 16+:  index set to 0 (sentinel for "unknown").
//     The system still works — the share data contains the Shamir
//     x-coordinate needed for Combine(). The UI just can't identify
//     which specific friend this share belongs to.
//
// Checksum: lower 7 bits of SHA-256(data_bytes)[0].
//   - Catches transpositions, word ordering mistakes, and typos that
//     happen to be valid BIP39 words.
//   - False positive rate: 1/128 (~0.8%).
const (
	word25IndexBits    = 4
	word25CheckBits    = 7
	word25MaxIndex     = (1 << word25IndexBits) - 1 // 15
	word25CheckMask    = (1 << word25CheckBits) - 1  // 0x7F
)

// word25Checksum computes the 7-bit checksum for the 25th word.
// It hashes the raw share data bytes and returns the lower 7 bits of byte 0.
func word25Checksum(data []byte) int {
	h := sha256.Sum256(data)
	return int(h[0]) & word25CheckMask
}

// word25Encode packs a share index and data checksum into an 11-bit BIP39 word index.
func word25Encode(shareIndex int, data []byte) int {
	idx := shareIndex
	if idx > word25MaxIndex {
		idx = 0 // sentinel: index not representable in 4 bits
	}
	check := word25Checksum(data)
	return (idx << word25CheckBits) | check
}

// word25Decode unpacks the 25th word's 11-bit value into index and checksum.
func word25Decode(val int) (index int, checksum int) {
	return val >> word25CheckBits, val & word25CheckMask
}

// Words returns this share's data encoded as 25 BIP39 words.
// The first 24 words encode the share data (33 bytes = 264 bits, 11 bits per word).
// The 25th word packs 4 bits of share index + 7 bits of checksum (see word25 layout above).
// Returns nil for v1 shares, which use base64-encoded data unsuitable for word encoding.
func (s *Share) Words() []string {
	if s.Version < 2 {
		return nil
	}
	words := EncodeWords(s.Data)
	bip39Idx := word25Encode(s.Index, s.Data)
	words = append(words, bip39English[bip39Idx])
	return words
}

// DecodeShareWords decodes 25 BIP39 words into share data and index.
// The first 24 words are decoded to bytes; the 25th word carries index + checksum.
// Returns index=0 if the share index was > 15 (the sentinel value).
// Returns an error if the checksum doesn't match (wrong word order, typos, etc.).
func DecodeShareWords(words []string) (data []byte, index int, err error) {
	if len(words) < 2 {
		return nil, 0, fmt.Errorf("need at least 2 words")
	}

	// Look up the 25th word in the BIP39 list
	lastWord := strings.ToLower(strings.TrimSpace(words[len(words)-1]))
	initWordIndex()

	bip39Idx, ok := wordIndex[lastWord]
	if !ok {
		suggestion := SuggestWord(lastWord)
		if suggestion != "" {
			return nil, 0, fmt.Errorf("word %d %q not recognized — did you mean %q?", len(words), lastWord, suggestion)
		}
		return nil, 0, fmt.Errorf("word %d %q not recognized", len(words), lastWord)
	}

	// Decode the data words (all but the last)
	data, err = DecodeWords(words[:len(words)-1])
	if err != nil {
		return nil, 0, err
	}

	// Unpack index and checksum from the 25th word
	index, expectedCheck := word25Decode(bip39Idx)

	// Verify checksum against the decoded data
	actualCheck := word25Checksum(data)
	if actualCheck != expectedCheck {
		return nil, 0, fmt.Errorf("word checksum failed — check word order and spelling")
	}

	return data, index, nil
}

// SuggestWord finds the closest BIP39 word by Levenshtein distance (max 2).
// Returns empty string if no close match is found.
func SuggestWord(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}

	bestWord := ""
	bestDist := 3 // only suggest if distance <= 2

	for _, w := range bip39English {
		d := levenshtein(input, w)
		if d < bestDist {
			bestDist = d
			bestWord = w
		}
		if d == 0 {
			return w // exact match
		}
	}

	return bestWord
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Use single-row optimization
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = curr
	}

	return prev[len(b)]
}
