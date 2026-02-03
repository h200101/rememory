# Security Audit: ReMemory

**Audit Date:** 2026-02-03
**Scope:** Full codebase review for homelab/self-hosted use
**Commit:** c5e4c49 (main branch)
**Intended Audience:** Security-conscious users evaluating whether to run this software

---

## TL;DR

ReMemory is a CLI tool that encrypts files and splits the decryption key among friends using Shamir's Secret Sharing. Here's what matters for running it at home:

| Concern | Status | How to Verify |
|---------|--------|---------------|
| Phone home / telemetry | None | [Section 2.1](#21-no-network-calls-in-cli) |
| Analytics | None | [Section 2.2](#22-no-analytics-in-browser-tool) |
| External dependencies at runtime | None | [Section 2.3](#23-fully-offline-recovery) |
| Cryptography | Standard libraries (age, HashiCorp Vault) | [Section 3](#3-cryptographic-verification) |
| Data leaves your machine | Only what you manually distribute | [Section 4](#4-data-flow-analysis) |

**Bottom line (at commit c5e4c49):** I found no telemetry, no unexpected network access in the CLI, and the recovery bundle is self-contained for offline use. Cryptography relies on [age](https://github.com/FiloSottile/age) and [HashiCorp Vault](https://github.com/hashicorp/vault) implementations rather than custom primitives.

---

## 1. Quick Verification Commands

Run these yourself to verify the claims in this audit. All commands assume you're in the repository root.

### 1.1 Check for Network-Related Imports

```bash
# Search for network packages in Go code
grep -r "net/http\|net/url\|net\.Dial\|http\.Get\|http\.Post" --include="*.go" .

# Expected: No matches in application code
# (You may see matches in vendor/test code, which is fine)
```

### 1.2 Check for Telemetry/Analytics Patterns

```bash
# Common telemetry patterns (exclude node_modules which is test tooling)
grep -ri "telemetry\|analytics\|tracking\|sentry\|bugsnag\|datadog\|newrelic\|mixpanel\|segment\|amplitude" --include="*.go" --include="*.js" . --exclude-dir=node_modules

# Expected: No matches
```

### 1.3 Check for Hardcoded External URLs

```bash
# Look for URLs in Go code
grep -rE "https?://" --include="*.go" . | grep -v "_test.go" | grep -v "github.com/eljojo/rememory"

# Expected: Only GitHub release URL for documentation purposes
# (used in bundle README to tell users where to download CLI)
```

### 1.4 Check JavaScript for External Requests

```bash
# Check the browser recovery tool for fetch/XHR
grep -n "fetch\(|XMLHttpRequest|\.ajax\(|sendBeacon" internal/html/assets/app.js

# Expected output (only one match, for local WASM):
#   50:        fetch('recover.wasm'),

# Verify it's a relative path, not an external URL:
grep -A1 "fetch(" internal/html/assets/app.js
# Should show: fetch('recover.wasm') — no http:// or https://
```

### 1.5 Check for Shell Command Execution

```bash
# Check for os/exec usage (could shell out to network tools)
grep -rn "os/exec" --include="*.go" . | grep -v _test.go

# Expected: No matches

# Check for common network tool invocations
grep -rn 'exec\.Command\|"curl"\|"wget"\|"nc"\|"ssh"\|"scp"' --include="*.go" . | grep -v _test.go

# Expected: No matches
```

### 1.6 Verify Dependencies

```bash
# List all direct dependencies
go list -m all | head -20

# Verify checksums match upstream
go mod verify

# Expected: "all modules verified"
```

---

## 2. Network Isolation Verification

### 2.1 No Network Calls in CLI

The CLI makes zero network requests. Verify by examining imports:

```bash
# Check what the main packages import
go list -f '{{.ImportPath}}: {{.Imports}}' ./cmd/... ./internal/... 2>/dev/null | grep -E "net/http|net\""

# Expected: Empty output
```

**What the CLI does:**
- Reads files from disk
- Writes files to disk
- That's it

### 2.2 No Analytics in Browser Tool

The `recover.html` browser tool is fully self-contained:

```bash
# Check for any external script loading
grep -E "<script.*src=|import.*from ['\"]http" internal/html/assets/*.html internal/html/assets/*.js

# Expected: No external scripts
```

```bash
# Check for tracking pixels or beacons
grep -iE "beacon|pixel|track|gtag|ga\(|fbq\(|_paq" internal/html/assets/*.js internal/html/assets/*.html

# Expected: No matches
```

**What the browser tool does:**
- Loads WASM from embedded base64 (no network)
- Reads files you drag onto it (local FileReader API)
- Decrypts in browser memory
- Offers download (local blob URL)

### 2.3 Fully Offline Recovery

The recovery tool is designed to work without internet, even decades from now:

```bash
# Verify WASM fallback uses embedded binary, not external fetch
grep -n "WASM_BINARY" internal/html/assets/app.js

# Expected output shows fallback to embedded base64:
#   62:      if (typeof WASM_BINARY !== 'undefined') {
#   65:          const bytes = base64ToArrayBuffer(WASM_BINARY);
```

The code first tries to `fetch('recover.wasm')` which works if files are extracted. When that fails (as it does in the bundled HTML), it falls back to the base64-embedded WASM binary. **No external server is ever contacted.**

Source: [`internal/html/assets/app.js` lines 46-76](https://github.com/eljojo/rememory/blob/c5e4c49/internal/html/assets/app.js#L46-L76)

### 2.4 Content Security Policy Ready

The HTML has no inline event handlers or eval():

```bash
# Check for unsafe inline patterns
grep -E "onclick=|onerror=|onload=|eval\(|Function\(" internal/html/assets/*.html internal/html/assets/*.js

# Expected: No matches
```

---

## 3. Cryptographic Verification

### 3.1 Encryption Library

**Library:** [filippo.io/age](https://github.com/FiloSottile/age) v1.3.1

```bash
# Verify version
grep "filippo.io/age" go.mod

# Verify checksum
grep "filippo.io/age" go.sum
```

**Expected output:**
```
filippo.io/age v1.3.1
filippo.io/age v1.3.1 h1:hbzdQOJkuaMEpRCLSN1/C5DX74RPcNCk6oqhKMXmZi0=
```

**About age:**
- Created by Filippo Valsorda (former Go security lead, ex-Cloudflare)
- Uses ChaCha20-Poly1305 for encryption
- scrypt for key derivation from passphrase
- [Formal specification](https://github.com/C2SP/C2SP/blob/main/age.md)
- Widely used and audited

### 3.2 Secret Sharing Library

**Library:** [github.com/hashicorp/vault](https://github.com/hashicorp/vault) v1.21.2 (shamir package only)

```bash
# Verify version
grep "hashicorp/vault" go.mod

# Verify checksum
grep "hashicorp/vault" go.sum
```

**Expected output:**
```
github.com/hashicorp/vault v1.21.2
github.com/hashicorp/vault v1.21.2 h1:t6/vAAhgGvKukkIAQBUPenvfLiJ5oUm8CmOMa6tgUYQ=
```

**About the Shamir implementation:**
- Part of HashiCorp Vault (enterprise secrets management)
- Shamir's Secret Sharing over GF(2^8)
- Mathematical guarantee: k-1 shares reveal **zero** information

### 3.3 Passphrase Generation

```bash
# Check entropy source
grep -n "crypto/rand" internal/crypto/passphrase.go

# Check byte count (should be 32 = 256 bits)
grep -n "DefaultPassphraseBytes" internal/crypto/passphrase.go
```

**Expected:**
- Uses `crypto/rand` (OS CSPRNG: `/dev/urandom` on Linux)
- 32 bytes = 256 bits of entropy
- Base64 encoded for handling

### 3.4 Integrity Checks

```bash
# Verify constant-time comparison is used
grep -n "subtle.ConstantTimeCompare" internal/core/hash.go
```

This prevents timing attacks on hash verification.

---

## 4. Data Flow Analysis

### 4.1 What Data Goes Where

```
Your Machine (sealing)              Friends' Machines (recovery)
─────────────────────              ────────────────────────────

manifest/
    └─ your-secrets.txt
           │
           ▼
    ┌──────────────┐
    │ tar.gz       │ (in memory only, never written to disk)
    └──────────────┘
           │
           ▼
    ┌──────────────┐
    │ age encrypt  │ (256-bit random passphrase)
    └──────────────┘
           │
           ├─────────────────────► MANIFEST.age (encrypted, safe to distribute)
           │
           ▼
    ┌──────────────┐
    │ Shamir split │
    └──────────────┘
           │
           ├──► Share 1 ──────────► Alice's bundle
           ├──► Share 2 ──────────► Bob's bundle
           └──► Share 3 ──────────► Carol's bundle
                                           │
                                           ▼
                                   ┌──────────────┐
                                   │ recover.html │
                                   │ (browser)    │
                                   └──────────────┘
                                           │
                                   [k shares collected]
                                           │
                                           ▼
                                   Passphrase reconstructed
                                           │
                                           ▼
                                   MANIFEST.age decrypted
                                           │
                                           ▼
                                   Files downloaded locally
```

### 4.2 Verify Passphrase Never Written to Disk

```bash
# Search for passphrase being written to files
grep -n "WriteFile.*passphrase\|passphrase.*WriteFile" internal/cmd/*.go

# Expected: No matches
```

The passphrase exists only in memory during `seal`, then is split and discarded.

### 4.3 Verify Share Permissions

```bash
# Check file permissions for shares
grep -n "WriteFile.*0600\|0600.*WriteFile" internal/cmd/seal.go
```

**Expected:** Share files are written with `0600` (owner read/write only).

### 4.4 What's in a Bundle

Each friend receives a ZIP containing:

| File | Contents | Sensitive? |
|------|----------|------------|
| README.txt | Instructions + their share | Yes (their share only) |
| README.pdf | Same as above, printable | Yes (their share only) |
| MANIFEST.age | Encrypted archive | No (encrypted) |
| recover.html | Self-contained recovery tool | No |

A single bundle is useless without threshold-1 other shares.

---

## 5. Attack Surface Analysis

### 5.1 Path Traversal Protection

```bash
# Check for path traversal defenses
grep -n "\.\./" internal/core/archive.go internal/manifest/archive.go
grep -n "filepath.Clean" internal/manifest/archive.go
```

**Defenses:**
1. Regex blocks `..` in paths: `internal/core/archive.go:45`
2. Path resolution check: `internal/manifest/archive.go:183`

### 5.2 Zip Bomb Protection

```bash
# Check size limits
grep -n "MaxFileSize\|MaxTotalSize" internal/core/archive.go
```

**Expected:**
- Per-file limit: 100 MB
- Total limit: 1 GB

### 5.3 Symlink Handling

```bash
# Check symlink behavior
grep -n "ModeSymlink\|TypeSymlink" internal/manifest/archive.go internal/core/archive.go
```

**Expected:** Symlinks are skipped with warnings (not followed).

### 5.4 Input Validation

```bash
# Check input length limits
grep -n "len.*>" internal/cmd/init.go | head -10
```

Friend names, emails, phone numbers have length limits to prevent abuse.

---

## 6. Building from Source

If you don't trust pre-built binaries:

```bash
# Clone
git clone https://github.com/eljojo/rememory
cd rememory

# Verify you're at the audited commit
git log --oneline -1
# Should show: c5e4c49

# Build
make build

# Run tests
make test

# The binary is now at ./rememory
```

### 6.1 Reproducible Builds

```bash
# Nix users can build reproducibly
nix build

# Or use the flake
nix develop
make build
```

### 6.2 Cross-Compile

```bash
# Build for different platforms
make build-all

# Outputs in dist/
ls dist/
```

---

## 7. What This Audit Did NOT Cover

For full transparency:

- **Cryptographic implementation correctness** of age/vault libraries (deferred to their maintainers and auditors)
- **Side-channel attacks** beyond timing (power analysis, EM emissions)
- **Supply chain attacks** on Go modules (mitigated by go.sum but not eliminated)
- **Browser security** beyond basic sandbox assumptions
- **Hardware security** (HSM integration, secure enclaves)

---

## 8. Summary for Homelab Users

### Safe to Run If:

- You want offline, self-hosted secret sharing
- You don't want cloud dependencies
- You want to verify the code yourself
- You're comfortable with CLI tools

### Trust Decisions You're Making:

1. **Trust in age encryption** — widely used, created by respected cryptographer
2. **Trust in HashiCorp's Shamir** — enterprise-grade, battle-tested
3. **Trust in your friends** — they can decrypt if threshold cooperate
4. **Trust your device** — passphrase is in memory during seal

### What You're NOT Trusting:

- Any cloud service
- Any external server
- Any analytics provider
- The continued existence of this project
- The internet (for recovery)

---

## 9. Verification Checklist

Run through these commands to verify the audit claims yourself:

```bash
# 1. No network imports
grep -r "net/http" --include="*.go" . | grep -v _test.go | wc -l
# Expected: 0

# 2. No os/exec (no shelling out)
grep -r "os/exec" --include="*.go" . | grep -v _test.go | wc -l
# Expected: 0

# 3. No telemetry (excluding test tooling)
grep -ri "telemetry\|analytics" --include="*.go" --include="*.js" . --exclude-dir=node_modules | wc -l
# Expected: 0

# 4. Dependencies verified
go mod verify && echo "OK"
# Expected: OK

# 5. Age version
grep "filippo.io/age v" go.mod
# Expected: v1.3.1

# 6. Vault version
grep "hashicorp/vault v" go.mod
# Expected: v1.21.2

# 7. Passphrase entropy
grep "DefaultPassphraseBytes = " internal/crypto/passphrase.go
# Expected: 32

# 8. CSPRNG used
grep "crypto/rand" internal/crypto/passphrase.go | wc -l
# Expected: 1 or more

# 9. Constant-time comparison
grep "ConstantTimeCompare" internal/core/hash.go | wc -l
# Expected: 1

# 10. Share permissions restrictive
grep "0600" internal/cmd/seal.go | wc -l
# Expected: 1 or more

# 11. Path traversal protection
grep -c "\\.\\." internal/core/archive.go
# Expected: 1 or more

# 12. JS fetch is local only
grep -n "fetch(" internal/html/assets/app.js
# Expected: only 'recover.wasm' (relative path, no http)
```

---

## References

- [age encryption - GitHub](https://github.com/FiloSottile/age)
- [age specification](https://github.com/C2SP/C2SP/blob/main/age.md)
- [HashiCorp Vault shamir package](https://pkg.go.dev/github.com/hashicorp/vault/shamir)
- [Shamir's Secret Sharing - Wikipedia](https://en.wikipedia.org/wiki/Shamir%27s_secret_sharing)

---

*This audit was performed through static code analysis. Verify the claims yourself using the commands provided. For mission-critical use, consider additional review by a security professional.*
