# ReMemory — Protocol & Implementation Plan

**Codename:** Orange Plan
**Version:** 0.2 — Refined after research
**Last updated:** 2026-02-02

---

## One-line summary

A CLI tool that encrypts a small manifest of "keys to the kingdom" with `age`, splits the passphrase via Shamir's Secret Sharing, and packages self-contained recovery bundles for distribution to trusted friends.

---

## What this is (and isn't)

**IS:** A system to give trusted people the ability to reconstruct a single passphrase that decrypts a small file (~10MB) containing credentials, recovery codes, and a map of where everything lives. Think of it as a master index — not a backup of the data itself.

**IS NOT:** A vault, a backup system, a file sync tool, or a dead man's switch (though it can be composed with one later).

The encrypted cloud backups already exist. ReMemory is the last mile: getting the right people access to those backups when you can't do it yourself.

---

## Core architecture

```
┌───────────────────────────────────────────────────────┐
│                    YOU (setup time)                   │
│                                                       │
│  1. Write the manifest (guided by the tool)           │
│  2. `rememory seal` encrypts it with age + passphrase │
│  3. Passphrase is split into N shares (threshold K)   │
│  4. `rememory bundle` creates one ZIP per friend      │
│                                                       │
└──────────────┬────────────────────────────────────────┘
               │
               ▼
┌───────────────────────────────────────────────────────┐
│              BUNDLE (one per friend)                  │
│                                                       │
│  bundle-alice/                                        │
│  ├── README.pdf           ← human-readable guide      │
│  ├── MANIFEST.age         ← the encrypted payload     │
│  ├── SHARE.txt            ← this person's SSS share   │
│  ├── CONTACTS.txt         ← how to reach the others   │
│  ├── rememory-linux-amd64 ← static binary             │
│  ├── rememory-darwin-arm64                            │
│  ├── rememory-windows.exe                             │
│  └── recover.html         ← offline WASM fallback     │
│                                                       │
└───────────────────────────────────────────────────────┘
               │
               ▼
┌───────────────────────────────────────────────────────┐
│           RECOVERY (K friends cooperate)              │
│                                                       │
│  1. K friends come together (in person or remote)     │
│  2. Each provides their SHARE.txt                     │
│  3. `rememory recover` reconstructs the passphrase    │
│  4. Passphrase decrypts MANIFEST.age                  │
│  5. Manifest tells them what to do next               │
│                                                       │
└───────────────────────────────────────────────────────┘
```

---

## What goes in the manifest

The tool should guide the user through creating this with interactive prompts and a template. The manifest is a structured document (markdown or JSON) containing:

### Tier 1 — The skeleton key (unlocks everything else)
- **Password manager:** 1Password/Bitwarden master password, secret key, account URL
- **Primary email:** password, 2FA backup codes, recovery email/phone
- **Phone PIN / device passcodes**

### Tier 2 — Financial & critical accounts
- **Bank accounts:** institution, login, 2FA method, backup codes
- **Crypto wallets:** seed phrases, hardware wallet PINs, derivation paths, which chains
- **Cloud storage:** provider, login (or note "accessible via email recovery")
- **Borg backup:** repo location, encryption passphrase
- **Yubikeys**: physical location, PINs, PUKs

### Tier 3 — The map (where stuff lives)
- **What is backed up where:** "Photos → iCloud + Backblaze", "Documents → Google Drive + encrypted Syncthing to NAS"
- **Hardware locations:** "NAS is in the closet, password is in 1Password under 'Synology'"
- **Recovery sequences:** "To get into my laptop, first recover email, then use Apple ID recovery, then..."
- **Legal contacts:** lawyer name/phone, will location

### Tier 4 — Personal wishes (optional)
- What to do with social media accounts
- Who should have access to what
- Messages to specific people

### What to EXCLUDE
- Sensitive employment / NDA-covered information
- Client/patient confidential data
- Anything that should go through a lawyer instead
- The manifest itself should never be stored in a will (wills become public record)

---

## Technical decisions

### Language: Go

**Rationale:**
- Static binaries by default, no runtime dependencies — critical for bundles that must work in 10+ years
- Hashicorp Vault's SSS library is battle-tested Go code, directly importable
- `age` reference implementation is Go, can embed as library
- Cross-compilation is trivial: `GOOS=darwin GOARCH=arm64 go build`
- Simpler than Rust for a project of this scope; the crypto primitives are in libraries, not hand-rolled

**Rust alternative considered:** dsprenkels/sss has better side-channel resistance, and rage (Rust age) is solid. But Go's compilation story and library ecosystem are more practical here. The threat model doesn't include side-channel attacks on the tool itself (the attacker would need physical access to the machine during split/recover operations).

### Encryption: age (not GPG)

**Rationale:**
- Symmetric mode (`age -p`) with a passphrase is all we need
- No key management, no algorithm negotiation, no config files
- Modern crypto: XChaCha20-Poly1305, scrypt for key derivation
- The passphrase becomes the Shamir-split secret — simple, clean
- Implementations exist in Go (reference), Rust (rage), JS — recovery doesn't depend on a single codebase
- Spec is frozen and documented at age-encryption.org/v1

**GPG rejected because:** algorithm negotiation attack surface, backward-compatibility baggage, complexity that kills usability in emergency recovery scenarios. Friends don't need to understand public key infrastructure.

### Secret sharing: Hashicorp Vault's shamir package

**Rationale:**
- Extracted from Vault (32k+ stars), regularly audited
- Clean API: `Split(secret, parts, threshold)` → `[][]byte`
- GF(2^8) arithmetic, works on arbitrary byte strings
- Well-maintained, unlikely to disappear

**Considerations:**
- No integrity verification on shares (basic SSS limitation). Mitigated by: each share file includes a SHA-256 checksum of itself, and the tool verifies checksums before attempting reconstruction.
- No verifiable secret sharing (Feldman/Pedersen). Acceptable for this threat model — we trust share holders to be honest, the risk is share corruption/loss, not adversarial share submission.

### Default threshold: 3-of-5

**Rationale:**
- 5 trusted friends is realistic to maintain over decades
- Threshold of 3 means: any 2 people colluding can't recover; losing 2 shares doesn't lock you out
- Can be configured: 2-of-3 for simpler setups, 4-of-7 for higher security

### Bundle format

Each friend receives a ZIP file (or a USB drive containing):

| File | Purpose | Format |
|------|---------|--------|
| `README.pdf` | Human instructions: what this is, when to use it, how to contact others, step-by-step recovery | PDF/A-1a (archival, self-contained) |
| `README.txt` | Same content as plaintext fallback | UTF-8 text |
| `MANIFEST.age` | The encrypted payload | age binary format |
| `SHARE-3of5-alice.txt` | This person's Shamir share + metadata | Base64-encoded, with checksum |
| `CONTACTS.txt` | Names + contact info for all other share holders | Plaintext |
| `METADATA.json` | Tool version, creation date, threshold, total shares, share index | JSON |
| `rememory-*` | Static binaries for linux/mac/windows (amd64 + arm64) | ~10MB each |
| `recover.html` | Offline single-page app: paste shares → get passphrase → paste into age | HTML + WASM |
| `checksums.txt` | SHA-256 of every file in the bundle | Plaintext |

**Total bundle size estimate:** 50-70MB (dominated by the 5 static binaries). The actual secret payload is <10MB.

---

## CLI design

```
rememory init                    # Create a new manifest from guided prompts/template
rememory edit                    # Open manifest for editing
rememory seal                    # Encrypt manifest → MANIFEST.age (prompts for passphrase or generates one)
rememory split                   # Split passphrase into shares according to manifest config
rememory bundle [--output ./bundles/]       # Generate per-person ZIP bundles
rememory verify                  # Test that K shares reconstruct correctly
rememory recover <share1> <share2> ...      # Reconstruct passphrase, decrypt manifest
rememory rotate                  # Generate new passphrase, re-encrypt, re-split (for periodic updates)
rememory status                  # Show current config: who holds shares, when last rotated, when to rotate next
rememory verify-bundle <bundle.zip>   # Verify integrity of a given bundle
```

# Composed workflow
rememory init && rememory seal && rememory split && rememory bundle
# Or: rememory seal-and-bundle (all-in-one)

---

## Open questions resolved

### Should others really have access to my data in case of death?
**Yes, but scoped.** The manifest controls what they can access. You're not giving them your data — you're giving them a map + the key to the front door. What's behind each subsequent door is up to each service's own access controls. Pair this with proper legal estate planning (will, digital executor designation).

### How to expire old keys?
**Rotation, not expiration.** `rememory rotate` generates a new passphrase, re-encrypts the manifest, creates new shares. Old shares become useless because they reconstruct the old passphrase, which no longer decrypts anything. Old bundles should be physically destroyed/deleted. The CONTACTS.txt instructs holders to destroy old bundles upon receiving new ones.

### How will these people contact each other?
**CONTACTS.txt in every bundle.** Contains name, phone, email, and optionally a secondary contact method for each share holder. Updated on each rotation. The README.pdf instructs holders to contact each other if they believe recovery is needed.

### Should the system be online or offline?
**Offline-first, online-optional.** The core system is completely offline — no servers, no accounts, no services to shut down. A future extension could add a dead man's switch (email-based check-in) that notifies share holders when to act, but the recovery mechanism itself must work without internet.

---

## Threat model & failure modes

### What we defend against
| Threat | Mitigation |
|--------|------------|
| You lose memory / get incapacitated | Friends reconstruct passphrase, follow the map |
| You die | Same as above; legal executor + friends cooperate |
| A single friend is compromised/adversarial | Threshold prevents single-share recovery |
| Two friends collude (in 3-of-5) | Need 3 — still safe with 2 colluding |
| Friend loses their share | 3-of-5 tolerates losing 2 shares |
| Software doesn't exist in 10 years | Static binaries + WASM fallback + recovery is just "get passphrase, decrypt with age" — age will exist, or the spec is simple enough to reimplement |
| USB drive degrades | Recommend M-Disc + refreshed USB every 2-3 years + encrypted cloud copy |

### What we DON'T defend against
| Threat | Why | Mitigation |
|--------|-----|------------|
| Threshold of friends all collude | By design — if 3 of your 5 most trusted people conspire, you have bigger problems | Choose friends from different social circles |
| Sophisticated side-channel attacks during split/recover | Tool runs on user's own machine; attacker needs physical/remote access | Use a trusted machine, air-gapped if paranoid |
| Quantum computing breaks age's crypto | Decades away for symmetric crypto (XChaCha20 is 256-bit) | age's symmetric mode is quantum-resistant |
| Long-term relationship drift | People move, lose touch, fall out over decades | Rotate every 2-3 years; replace holders as needed |
| Friends don't understand the instructions | | Invest heavily in README quality; test with real people |

### Known SSS implementation risks (from research)
- **No integrity verification on basic SSS:** a corrupted share produces a wrong passphrase silently. Mitigated by: include a checksum of the original passphrase (or a verification hash) in METADATA.json so recovery can confirm success.
- **Secret exists in memory during split and recovery:** unavoidable single point of failure at generation time. Accept this risk.
- **No share revocation without full rotation:** can't invalidate one person's share without re-splitting. This is fine — rotation is the answer.

---

## Existing tools to use vs. build

### USE off the shelf
| Component | Tool | Notes |
|-----------|------|-------|
| Encryption | `age` (filipposottile/age) | Use as Go library, not CLI dependency |
| SSS | `hashicorp/vault/shamir` | Import the package directly |
| PDF generation | Go library (e.g., `jung-kurt/gofpdf` or `go-pdf/fpdf`) | For README.pdf in bundles |
| WASM | Go's built-in WASM target | For recover.html fallback |

### BUILD custom
| Component | Why |
|-----------|-----|
| Manifest template & guided init | No existing tool does this |
| Bundle packaging | Unique to our format |
| Share file format with checksums | Needs to be human-readable + verifiable |
| Recovery verification | Need to test round-trip without exposing the secret |
| CLI orchestration | Glue between components |

### DEFER (future extensions)
| Feature | Tool/Pattern | When |
|---------|-------------|------|
| Dead man's switch | storopoli/dead-man-switch (Rust) or custom Go service | v2 — after core works |
| Timelock encryption | drand/tlock | v2 — auto-release after time period |
| Proactive share refresh | PSS protocol | v3 — only if rotation logistics become painful |
| Web UI for recovery | Expand recover.html | v2 — after testing with real users |
| Automated yearly testing | Cron + email reminders | v1.1 |

---

## Implementation phases

### Phase 1 — Core (MVP)
1. `rememory init` — generate manifest template, guide user through filling it
2. `rememory seal` — encrypt with age (symmetric passphrase, auto-generated)
3. `rememory split` — Shamir split the passphrase
4. `rememory recover` — reconstruct + decrypt
5. `rememory verify` — round-trip test
7. Comprehensive tests: split → recover for every valid threshold combination

### Phase 2 — Distribution
7. `rememory bundle` — generate per-person ZIPs with binaries + docs
8. Cross-compile for 5 targets
9. WASM recovery fallback (recover.html)
10. PDF README generation (human-readable recovery guide)
11. `rememory status` — show config, rotation reminders
12. `rememory verify-bundle` — command used by share holders to check integrity of their bundle, and that code runs well. also possible from recover.html.

### Phase 3 — Operations
13. `rememory rotate` — re-key and re-bundle
14. Share holder management (add/remove people, update contacts)
15. Yearly check/reminder system (could be as simple as a calendar invite generator)

### Phase 4 — Extensions (future)
16. Dead man's switch integration
17. Timelock encryption layer
18. Encrypted cloud distribution option (share holders get a link instead of a USB)

---

## Testing strategy

The most important property: **recovery always works.** Every code path that touches crypto must be tested with round-trip verification.

- **Unit tests:** SSS split/combine for all valid threshold/share combinations (2-of-3 through 10-of-10)
- **Integration tests:** full `seal → split → bundle → recover` pipeline
- **Fuzz tests:** corrupted shares should fail gracefully with clear error messages, never produce wrong output silently
- **Cross-platform tests:** recovery works on all 5 target platforms
- **WASM tests:** recover.html produces identical results to CLI
- **Archival tests:** bundles from old versions can still be recovered (backward compatibility contract)
- **Human tests:** give a bundle to a real person, see if they can recover without your help

---

## Project structure (proposed)

```
rememory/
├── cmd/
│   └── rememory/
│       └── main.go
├── internal/
│   ├── manifest/      # template, guided init, validation
│   ├── vault/         # age encryption wrapper
│   ├── shamir/        # SSS wrapper around hashicorp/vault/shamir
│   ├── bundle/        # ZIP packaging, binary embedding, PDF generation
│   ├── share/         # share file format, checksums, metadata
│   └── wasm/          # WASM build of recovery logic
├── templates/
│   ├── manifest.md.tmpl
│   └── readme.md.tmpl
├── docs/
│   └── PROTOCOL.md    # specification of file formats, crypto choices
├── test/
│   ├── integration/
│   └── fixtures/
├── go.mod
├── go.sum
├── Makefile           # cross-compile, WASM build, test, release
└── README.md
```

---

## Key references from research

### Libraries to use
- **age:** github.com/FiloSottile/age — reference Go implementation, use as library
- **SSS:** github.com/hashicorp/vault/shamir — battle-tested, extractable package

### Closest existing projects (study, don't fork)
- **jesseduffield/horcrux** — Go, file splitting with SSS, good UX reference
- **storopoli/dead-man-switch** — Rust, for future dead man's switch integration
- **drand/tlock** — Go, timelock encryption for future auto-release feature

### Critical reading
- Casa blog: "Shamir's Secret Sharing security shortcomings" — real-world failure catalog
- Trail of Bits: SSS vulnerability disclosures (Binance tss-lib bugs)
- Vitalik Buterin: "Why we need wide adoption of social recovery wallets" — guardian selection philosophy
- SLIP-39 spec (Trezor) — standardized Shamir for seed phrases, checksummed shares

### Security warnings from research
- HTC Exodus phone: PRNG bug allowed single shareholder to recover seed meant for threshold
- Armory wallet: repeated hashing instead of randomness meant first share + any other = recovery
- Basic SSS has no integrity protection — always verify reconstruction with a known checksum
