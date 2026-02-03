package html

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// FriendInfo holds friend contact information for the UI.
type FriendInfo struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// PersonalizationData holds the data to personalize recover.html for a specific friend.
type PersonalizationData struct {
	Holder       string       `json:"holder"`       // This friend's name
	HolderShare  string       `json:"holderShare"`  // This friend's encoded share
	OtherFriends []FriendInfo `json:"otherFriends"` // List of other friends
	Threshold    int          `json:"threshold"`    // Required shares (K)
	Total        int          `json:"total"`        // Total shares (N)
}

// GenerateRecoverHTML creates the complete recover.html with all assets embedded.
// wasmBytes should be the compiled recover.wasm binary.
// version is the rememory version string.
// githubURL is the URL to download CLI binaries.
// personalization can be nil for a generic recover.html, or provided to personalize for a specific friend.
func GenerateRecoverHTML(wasmBytes []byte, version, githubURL string, personalization *PersonalizationData) string {
	html := recoverHTMLTemplate

	// Embed styles
	html = strings.Replace(html, "{{STYLES}}", stylesCSS, 1)

	// Embed wasm_exec.js
	html = strings.Replace(html, "{{WASM_EXEC}}", wasmExecJS, 1)

	// Embed app.js
	html = strings.Replace(html, "{{APP_JS}}", appJS, 1)

	// Embed WASM as base64
	wasmB64 := base64.StdEncoding.EncodeToString(wasmBytes)
	html = strings.Replace(html, "{{WASM_BASE64}}", wasmB64, 1)

	// Replace version and GitHub URL
	html = strings.Replace(html, "{{VERSION}}", version, 1)
	html = strings.Replace(html, "{{GITHUB_URL}}", githubURL, 1)

	// Embed personalization data as JSON (or null if not provided)
	var personalizationJSON string
	if personalization != nil {
		data, _ := json.Marshal(personalization)
		personalizationJSON = string(data)
	} else {
		personalizationJSON = "null"
	}
	html = strings.Replace(html, "{{PERSONALIZATION_DATA}}", personalizationJSON, 1)

	return html
}
