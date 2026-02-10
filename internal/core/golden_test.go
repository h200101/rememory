package core

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var generate = flag.Bool("generate", false, "regenerate golden test fixtures (writes to testdata/)")

// --- JSON fixture types ---

type goldenFixture struct {
	Version    int            `json:"version"`
	Passphrase string         `json:"passphrase"`
	Total      int            `json:"total"`
	Threshold  int            `json:"threshold"`
	Created    string         `json:"created"`
	Shares     []goldenShare  `json:"shares"`
	Manifest   goldenManifest `json:"manifest"`
}

type goldenShare struct {
	Index    int    `json:"index"`
	Holder   string `json:"holder"`
	DataHex  string `json:"data_hex"`
	Checksum string `json:"checksum"`
	PEM      string `json:"pem"`
	Compact  string `json:"compact"`
}

type goldenManifest struct {
	Files map[string]string `json:"files"`
}

// --- Constants for golden fixtures ---

const (
	// goldenPassphrase is a fixed base64url string (43 chars, represents 32 bytes).
	// This mimics the output of crypto.GeneratePassphrase(32) but is deterministic.
	goldenPassphrase = "dGhpc19pc19hX3Rlc3RfcGFzc3BocmFzZV92MV9nbGRu"

	// goldenCreated is the fixed timestamp for all golden shares.
	goldenCreated = "2025-01-01T00:00:00Z"
)

var goldenHolders = []string{"Alice", "Bob", "Carol", "David", "Eve"}

// goldenManifestFiles are the known files inside the golden test manifest.
var goldenManifestFiles = map[string]string{
	"manifest/README.md":  "# Golden Test Manifest\n\nThis is a test manifest for v1 golden fixtures.\n",
	"manifest/secret.txt": "The secret passphrase is: correct-horse-battery-staple\n",
}

// --- Helpers ---

func loadGoldenJSON(t *testing.T) goldenFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "v1-golden.json"))
	if err != nil {
		t.Fatalf("reading v1-golden.json: %v", err)
	}
	var golden goldenFixture
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("unmarshaling v1-golden.json: %v", err)
	}
	return golden
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	data, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decoding hex: %v", err)
	}
	return data
}

// combinations returns all k-element subsets of {0, 1, ..., n-1}.
func combinations(n, k int) [][]int {
	var result [][]int
	combo := make([]int, k)
	var gen func(start, depth int)
	gen = func(start, depth int) {
		if depth == k {
			dup := make([]int, k)
			copy(dup, combo)
			result = append(result, dup)
			return
		}
		for i := start; i < n; i++ {
			combo[depth] = i
			gen(i+1, depth+1)
		}
	}
	gen(0, 0)
	return result
}

// buildSortedTarGz creates a tar.gz with entries in sorted key order for determinism.
func buildSortedTarGz(files map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for _, name := range keys {
		content := files[name]
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0644,
			ModTime:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			Typeflag: tar.TypeReg,
		}); err != nil {
			return nil, fmt.Errorf("writing tar header for %q: %w", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, fmt.Errorf("writing tar content for %q: %w", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}
	return buf.Bytes(), nil
}

// --- Generator ---

// TestGenerateGoldenFixtures generates all golden test fixtures.
// Run once with: go test -v -run TestGenerateGoldenFixtures -generate ./internal/core/
func TestGenerateGoldenFixtures(t *testing.T) {
	if !*generate {
		t.Skip("skipping fixture generation (use -generate flag to regenerate)")
	}

	createdTime, err := time.Parse(time.RFC3339, goldenCreated)
	if err != nil {
		t.Fatalf("parsing created time: %v", err)
	}

	// Split the passphrase using Shamir's Secret Sharing
	rawShares, err := Split([]byte(goldenPassphrase), 5, 3)
	if err != nil {
		t.Fatalf("splitting passphrase: %v", err)
	}

	// Verify reconstruction before committing fixtures
	recovered, err := Combine(rawShares[:3])
	if err != nil {
		t.Fatalf("combining shares: %v", err)
	}
	if string(recovered) != goldenPassphrase {
		t.Fatal("share reconstruction failed — shares are broken")
	}

	// Build shares with fixed metadata
	shares := make([]*Share, 5)
	goldenShares := make([]goldenShare, 5)
	for i := 0; i < 5; i++ {
		share := &Share{
			Version:   1,
			Index:     i + 1,
			Total:     5,
			Threshold: 3,
			Holder:    goldenHolders[i],
			Created:   createdTime,
			Data:      rawShares[i],
			Checksum:  HashBytes(rawShares[i]),
		}
		shares[i] = share
		goldenShares[i] = goldenShare{
			Index:    share.Index,
			Holder:   share.Holder,
			DataHex:  hex.EncodeToString(share.Data),
			Checksum: share.Checksum,
			PEM:      share.Encode(),
			Compact:  share.CompactEncode(),
		}
	}

	// Build manifest archive
	archiveData, err := buildSortedTarGz(goldenManifestFiles)
	if err != nil {
		t.Fatalf("building tar.gz: %v", err)
	}

	// Encrypt manifest
	var encryptedBuf bytes.Buffer
	if err := Encrypt(&encryptedBuf, bytes.NewReader(archiveData), goldenPassphrase); err != nil {
		t.Fatalf("encrypting manifest: %v", err)
	}

	// Build fixture JSON
	fixture := goldenFixture{
		Version:    1,
		Passphrase: goldenPassphrase,
		Total:      5,
		Threshold:  3,
		Created:    goldenCreated,
		Shares:     goldenShares,
		Manifest: goldenManifest{
			Files: goldenManifestFiles,
		},
	}

	fixtureJSON, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatalf("marshaling fixture JSON: %v", err)
	}

	// Create directories
	bundleDir := filepath.Join("testdata", "v1-bundle")
	expectedDir := filepath.Join(bundleDir, "expected-output")
	if err := os.MkdirAll(expectedDir, 0755); err != nil {
		t.Fatalf("creating directories: %v", err)
	}

	// Write v1-golden.json
	jsonPath := filepath.Join("testdata", "v1-golden.json")
	if err := os.WriteFile(jsonPath, fixtureJSON, 0644); err != nil {
		t.Fatalf("writing %s: %v", jsonPath, err)
	}
	t.Logf("wrote %s", jsonPath)

	// Write share PEM files
	for _, share := range shares {
		filename := fmt.Sprintf("SHARE-%s.txt", strings.ToLower(share.Holder))
		sharePath := filepath.Join(bundleDir, filename)
		if err := os.WriteFile(sharePath, []byte(share.Encode()), 0644); err != nil {
			t.Fatalf("writing %s: %v", sharePath, err)
		}
		t.Logf("wrote %s", sharePath)
	}

	// Write MANIFEST.age
	manifestPath := filepath.Join(bundleDir, "MANIFEST.age")
	if err := os.WriteFile(manifestPath, encryptedBuf.Bytes(), 0644); err != nil {
		t.Fatalf("writing %s: %v", manifestPath, err)
	}
	t.Logf("wrote %s (%d bytes)", manifestPath, encryptedBuf.Len())

	// Write expected output files
	for name, content := range goldenManifestFiles {
		outPath := filepath.Join(expectedDir, name)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			t.Fatalf("creating dir for %s: %v", outPath, err)
		}
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			t.Fatalf("writing %s: %v", outPath, err)
		}
		t.Logf("wrote %s", outPath)
	}

	t.Log("Golden fixtures generated successfully.")
	t.Log("Commit the testdata/ directory. These fixtures must never be modified.")
}

// --- Golden tests ---

// TestGoldenV1ShareParsing parses each fixture share and verifies all fields match.
func TestGoldenV1ShareParsing(t *testing.T) {
	golden := loadGoldenJSON(t)

	for _, gs := range golden.Shares {
		t.Run(gs.Holder, func(t *testing.T) {
			// Read PEM file from v1-bundle/
			filename := fmt.Sprintf("SHARE-%s.txt", strings.ToLower(gs.Holder))
			pemData, err := os.ReadFile(filepath.Join("testdata", "v1-bundle", filename))
			if err != nil {
				t.Fatalf("reading %s: %v", filename, err)
			}

			// Parse
			share, err := ParseShare(pemData)
			if err != nil {
				t.Fatalf("ParseShare: %v", err)
			}

			// Verify all fields
			if share.Version != golden.Version {
				t.Errorf("version: got %d, want %d", share.Version, golden.Version)
			}
			if share.Index != gs.Index {
				t.Errorf("index: got %d, want %d", share.Index, gs.Index)
			}
			if share.Total != golden.Total {
				t.Errorf("total: got %d, want %d", share.Total, golden.Total)
			}
			if share.Threshold != golden.Threshold {
				t.Errorf("threshold: got %d, want %d", share.Threshold, golden.Threshold)
			}
			if share.Holder != gs.Holder {
				t.Errorf("holder: got %q, want %q", share.Holder, gs.Holder)
			}

			expectedCreated, _ := time.Parse(time.RFC3339, golden.Created)
			if !share.Created.Equal(expectedCreated) {
				t.Errorf("created: got %v, want %v", share.Created, expectedCreated)
			}

			expectedData := mustDecodeHex(t, gs.DataHex)
			if !bytes.Equal(share.Data, expectedData) {
				t.Errorf("data mismatch: got %x, want %s", share.Data, gs.DataHex)
			}

			if share.Checksum != gs.Checksum {
				t.Errorf("checksum: got %q, want %q", share.Checksum, gs.Checksum)
			}

			// Verify checksum integrity
			if err := share.Verify(); err != nil {
				t.Errorf("Verify: %v", err)
			}

			// Re-encode and compare PEM
			reEncoded := share.Encode()
			if reEncoded != gs.PEM {
				t.Errorf("PEM re-encode mismatch:\ngot:\n%s\nwant:\n%s", reEncoded, gs.PEM)
			}

			// Compact encode and compare
			compact := share.CompactEncode()
			if compact != gs.Compact {
				t.Errorf("compact: got %q, want %q", compact, gs.Compact)
			}

			// Compact round-trip (only fields that survive: Version, Index, Total, Threshold, Data, Checksum)
			decoded, err := ParseCompact(compact)
			if err != nil {
				t.Fatalf("ParseCompact: %v", err)
			}
			if !bytes.Equal(decoded.Data, share.Data) {
				t.Errorf("compact round-trip data mismatch")
			}
			if decoded.Version != share.Version {
				t.Errorf("compact round-trip version: got %d, want %d", decoded.Version, share.Version)
			}
		})
	}
}

// TestGoldenV1Combine combines threshold shares and verifies the passphrase.
func TestGoldenV1Combine(t *testing.T) {
	golden := loadGoldenJSON(t)

	if len(golden.Shares) < golden.Threshold {
		t.Fatalf("not enough shares in fixture: have %d, need %d", len(golden.Shares), golden.Threshold)
	}

	shareData := make([][]byte, golden.Threshold)
	for i := 0; i < golden.Threshold; i++ {
		shareData[i] = mustDecodeHex(t, golden.Shares[i].DataHex)
	}

	recovered, err := Combine(shareData)
	if err != nil {
		t.Fatalf("Combine: %v", err)
	}

	if string(recovered) != golden.Passphrase {
		t.Errorf("passphrase: got %q, want %q", string(recovered), golden.Passphrase)
	}
}

// TestGoldenV1CombineAllSubsets tries all valid k-of-n subsets.
func TestGoldenV1CombineAllSubsets(t *testing.T) {
	golden := loadGoldenJSON(t)

	allData := make([][]byte, len(golden.Shares))
	for i, gs := range golden.Shares {
		allData[i] = mustDecodeHex(t, gs.DataHex)
	}

	subsets := combinations(len(golden.Shares), golden.Threshold)
	expectedSubsets := 10 // C(5,3) = 10
	if len(subsets) != expectedSubsets {
		t.Fatalf("expected %d subsets, got %d", expectedSubsets, len(subsets))
	}

	for _, subset := range subsets {
		// Build a human-readable name like "1,2,3"
		indices := make([]string, len(subset))
		for i, idx := range subset {
			indices[i] = fmt.Sprintf("%d", golden.Shares[idx].Index)
		}
		name := strings.Join(indices, ",")

		t.Run(name, func(t *testing.T) {
			shareData := make([][]byte, len(subset))
			for i, idx := range subset {
				shareData[i] = allData[idx]
			}

			recovered, err := Combine(shareData)
			if err != nil {
				t.Fatalf("Combine: %v", err)
			}

			if string(recovered) != golden.Passphrase {
				t.Errorf("passphrase: got %q, want %q", string(recovered), golden.Passphrase)
			}
		})
	}
}

// TestGoldenV1Decrypt combines shares, decrypts the manifest, and verifies output.
func TestGoldenV1Decrypt(t *testing.T) {
	golden := loadGoldenJSON(t)

	// Parse 3 share files from the bundle
	shareNames := []string{"alice", "bob", "carol"}
	shareData := make([][]byte, len(shareNames))
	for i, name := range shareNames {
		filename := fmt.Sprintf("SHARE-%s.txt", name)
		pemData, err := os.ReadFile(filepath.Join("testdata", "v1-bundle", filename))
		if err != nil {
			t.Fatalf("reading %s: %v", filename, err)
		}

		share, err := ParseShare(pemData)
		if err != nil {
			t.Fatalf("ParseShare(%s): %v", filename, err)
		}

		if err := share.Verify(); err != nil {
			t.Fatalf("Verify(%s): %v", filename, err)
		}

		shareData[i] = share.Data
	}

	// Combine shares to recover passphrase
	recovered, err := Combine(shareData)
	if err != nil {
		t.Fatalf("Combine: %v", err)
	}
	passphrase := string(recovered)

	if passphrase != golden.Passphrase {
		t.Fatalf("passphrase mismatch: got %q, want %q", passphrase, golden.Passphrase)
	}

	// Read and decrypt MANIFEST.age
	manifestAge, err := os.ReadFile(filepath.Join("testdata", "v1-bundle", "MANIFEST.age"))
	if err != nil {
		t.Fatalf("reading MANIFEST.age: %v", err)
	}

	var decrypted bytes.Buffer
	if err := Decrypt(&decrypted, bytes.NewReader(manifestAge), passphrase); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	// Extract tar.gz
	files, err := ExtractTarGz(decrypted.Bytes())
	if err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no files extracted from manifest")
	}

	// Build a map of extracted files for easy lookup
	extracted := make(map[string]string)
	for _, f := range files {
		extracted[f.Name] = string(f.Data)
	}

	// Verify against the JSON fixture
	for name, expectedContent := range golden.Manifest.Files {
		got, ok := extracted[name]
		if !ok {
			t.Errorf("missing extracted file %q", name)
			continue
		}
		if got != expectedContent {
			t.Errorf("file %q: got %q, want %q", name, got, expectedContent)
		}
	}

	// Also verify against files on disk in expected-output/
	for _, f := range files {
		diskPath := filepath.Join("testdata", "v1-bundle", "expected-output", f.Name)
		diskContent, err := os.ReadFile(diskPath)
		if err != nil {
			t.Errorf("reading expected output %s: %v", diskPath, err)
			continue
		}
		if string(f.Data) != string(diskContent) {
			t.Errorf("file %q doesn't match expected-output on disk", f.Name)
		}
	}
}
