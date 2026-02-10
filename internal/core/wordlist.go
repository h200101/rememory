package core

import (
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

// Words returns this share's data encoded as 25 BIP39 words.
// The first 24 words encode the share data (33 bytes = 264 bits).
// The 25th word encodes the share index (1-based) as a BIP39 word.
func (s *Share) Words() []string {
	words := EncodeWords(s.Data)
	if s.Index >= 0 && s.Index < len(bip39English) {
		words = append(words, bip39English[s.Index])
	}
	return words
}

// DecodeShareWords decodes 25 BIP39 words into share data and index.
// The first 24 words are decoded to bytes; the 25th word gives the share index.
func DecodeShareWords(words []string) (data []byte, index int, err error) {
	if len(words) < 2 {
		return nil, 0, fmt.Errorf("need at least 2 words")
	}

	// The last word encodes the share index
	lastWord := strings.ToLower(strings.TrimSpace(words[len(words)-1]))
	initWordIndex()

	idx, ok := wordIndex[lastWord]
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

	return data, idx, nil
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
