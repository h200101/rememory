# ReMemory User Guide

This guide walks you through using ReMemory to create encrypted recovery bundles for your trusted friends.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Creating Your First Project](#creating-your-first-project)
- [Adding Your Secrets](#adding-your-secrets)
- [Sealing the Project](#sealing-the-project)
- [Creating Distribution Bundles](#creating-distribution-bundles)
- [Distributing to Friends](#distributing-to-friends)
- [What Your Friends Receive](#what-your-friends-receive)
- [Recovery Process](#recovery-process)
- [Verifying Bundles](#verifying-bundles)
- [Best Practices](#best-practices)
- [Project Structure](#project-structure)
- [Commands Reference](#commands-reference)

## Overview

ReMemory helps you:

1. Encrypt sensitive files with strong cryptography
2. Split the decryption key among trusted friends using Shamir's Secret Sharing
3. Create self-contained bundles that friends can use to recover your secrets

The key innovation is that recovery works **entirely offline in a browser**—no servers, no internet, no need for ReMemory to exist when recovery happens.

## Installation

### From GitHub Releases

Download the latest binary for your platform from [Releases](https://github.com/eljojo/rememory/releases).

### With Go

```bash
go install github.com/eljojo/rememory/cmd/rememory@latest
```

### With Nix

```bash
nix run github:eljojo/rememory
```

Or add to your flake:

```nix
{
  inputs.rememory.url = "github:eljojo/rememory";
  inputs.rememory.inputs.nixpkgs.follows = "nixpkgs";
}
```

## Creating Your First Project

Start by creating a new project:

```bash
rememory init my-recovery-2026
cd my-recovery-2026
```

You'll be prompted to configure your recovery scheme:

```
How many friends will hold shares? [5]: 5
How many shares needed to recover? [3]: 3

Friend 1:
  Name: Alice
  Email: alice@example.com
  Phone (optional): 555-1234

Friend 2:
  Name: Bob
  Email: bob@example.com
  Phone (optional):

Friend 3:
  Name: Carol
  Email: carol@example.com
  Phone (optional): 555-3456

...
```

### Choosing the Right Numbers

| Friends | Recommended Threshold | Notes |
|---------|----------------------|-------|
| 3 | 2 | Minimum viable setup |
| 5 | 3 | Good balance of security and availability |
| 7 | 4-5 | Higher security, requires more coordination |

**Rule of thumb:** Set threshold high enough that casual collusion is unlikely, but low enough that recovery is possible if 1-2 friends are unavailable.

## Adding Your Secrets

Place your sensitive files in the `manifest/` directory:

```bash
# Copy important files
cp ~/Documents/recovery-codes.txt manifest/
cp ~/Documents/crypto-seeds.txt manifest/
cp ~/Documents/important-passwords.txt manifest/

# Or create files directly
echo "The safe combination is 12-34-56" > manifest/notes.txt
echo "Bank account: 123456789" >> manifest/notes.txt
```

You can organize files in subdirectories:

```bash
mkdir -p manifest/crypto
mkdir -p manifest/accounts
cp ~/wallets/*.txt manifest/crypto/
cp ~/passwords/*.txt manifest/accounts/
```

### What to Include

Good candidates for ReMemory:
- Password manager recovery codes
- Cryptocurrency seeds/keys
- Important account credentials
- Instructions for loved ones
- Legal document locations
- Safe combinations

### What NOT to Include

- Files that change frequently (use ReMemory for static secrets)
- Extremely large files (bundles become unwieldy)
- Anything already backed up elsewhere with good recovery options

## Sealing the Project

Once your secrets are in place, seal the project:

```bash
rememory seal
```

This:
1. Generates a random 256-bit passphrase
2. Encrypts all files in `manifest/` using age encryption
3. Splits the passphrase into shares using Shamir's Secret Sharing
4. Verifies that recovery works correctly

```
Encrypting manifest...
  ✓ Created MANIFEST.age (2.3 KB)

Splitting passphrase (3 of 5)...
  ✓ Share for Alice: SHARE-alice.txt
  ✓ Share for Bob: SHARE-bob.txt
  ✓ Share for Carol: SHARE-carol.txt
  ✓ Share for David: SHARE-david.txt
  ✓ Share for Eve: SHARE-eve.txt

Verifying recovery...
  ✓ Successfully verified with shares: Alice, Bob, Carol

Sealed! Run 'rememory bundle' to create distribution packages.
```

## Creating Distribution Bundles

Generate the ZIP files to send to your friends:

```bash
rememory bundle
```

This creates a self-contained bundle for each friend:

```
output/bundles/
├── bundle-alice.zip  (5.4 MB)
├── bundle-bob.zip    (5.4 MB)
├── bundle-carol.zip  (5.4 MB)
├── bundle-david.zip  (5.4 MB)
└── bundle-eve.zip    (5.4 MB)
```

Each bundle is ~5 MB because it includes the complete recovery tool.

## Distributing to Friends

Send each friend their specific bundle. Methods:

- **Email** — Attach the ZIP file
- **Cloud storage** — Share via Dropbox, Google Drive, etc.
- **USB drive** — Physical handoff
- **Encrypted messaging** — Signal, WhatsApp, etc.

Tell your friends:
1. Keep the bundle somewhere safe (cloud backup, USB drive, etc.)
2. They cannot use it alone—they'll need to coordinate with others
3. A single share reveals nothing, but they should still keep it private

## What Your Friends Receive

Each bundle contains:

| File | Purpose |
|------|---------|
| `README.txt` | Instructions + their unique share + contact list for other holders |
| `README.pdf` | Same content, formatted for printing |
| `MANIFEST.age` | Your encrypted secrets (same in all bundles) |
| `recover.html` | Browser-based recovery tool (~5 MB, self-contained) |

The README.txt includes:

```
================================================================================
                          REMEMORY RECOVERY BUNDLE
                              For: Alice
================================================================================

!!  YOU CANNOT USE THIS FILE ALONE
    You will need help from other friends listed below.

!!  CONFIDENTIAL - DO NOT SHARE THIS FILE
    This document contains your secret share. Keep it safe.

    NOTA PARA HISPANOHABLANTES:
    Si no entiendes inglés, puedes usar ChatGPT u otra inteligencia artificial
    para que te ayude a entender estas instrucciones y recuperar los datos.

--------------------------------------------------------------------------------
WHAT IS THIS?
--------------------------------------------------------------------------------
This bundle allows you to help recover encrypted secrets.
You are one of 5 trusted friends who hold pieces of the recovery key.
At least 3 of you must cooperate to decrypt the contents.

--------------------------------------------------------------------------------
OTHER SHARE HOLDERS (contact to coordinate recovery)
--------------------------------------------------------------------------------
Bob - bob@example.com - 555-2345
Carol - carol@example.com
David - david@example.com - 555-4567
Eve - eve@example.com

--------------------------------------------------------------------------------
HOW TO RECOVER (PRIMARY METHOD - Browser)
--------------------------------------------------------------------------------
1. Open recover.html in any modern browser
2. Drag and drop this README.txt file
3. Collect shares from other friends (they drag their README.txt too)
4. Once you have enough shares, the tool will decrypt automatically
5. Download the recovered files

Works completely offline - no internet required!

--------------------------------------------------------------------------------
YOUR SHARE
--------------------------------------------------------------------------------
-----BEGIN REMEMORY SHARE-----
Version: 1
Index: 1
Total: 5
Threshold: 3
Holder: Alice
...
-----END REMEMORY SHARE-----
```

## Recovery Process

### Browser Recovery (Recommended)

When your friends need to recover your secrets:

1. One friend opens `recover.html` from their bundle in any modern browser
2. They drag and drop their `README.txt` file onto the page
3. Other friends send their `README.txt` files (via email, messaging, etc.)
4. As each share is added, the progress updates
5. Once threshold is met (e.g., 3 of 5), decryption happens automatically
6. Download the recovered files

**Key points:**
- Works completely offline—no internet required
- No data leaves the browser
- Works on Chrome, Firefox, Safari, Edge
- Friends can be in different locations; they just need to share their README.txt files

### CLI Recovery (Fallback)

If the browser tool doesn't work:

```bash
# Download rememory from GitHub releases, then:
rememory recover \
  --shares alice-readme.txt,bob-readme.txt,carol-readme.txt \
  --manifest MANIFEST.age \
  --output recovered/
```

## Verifying Bundles

Before distributing, verify your bundles are valid:

```bash
rememory verify-bundle output/bundles/bundle-alice.zip
```

This checks:
- All required files are present
- Checksums match
- The embedded share is valid

You can also verify bundles you receive from others to ensure they haven't been corrupted.

## Best Practices

### Choosing Friends

- **Longevity** — Pick people likely to be reachable in 5-10+ years
- **Geographic diversity** — Don't put all friends in the same disaster zone
- **Technical ability** — Mix is fine; the tool is designed for everyone
- **Relationships** — Consider if they'll cooperate with each other
- **Trust** — While a single share reveals nothing, you're trusting them with responsibility

### Security Considerations

- **Keep your sealed project secure** — The passphrase is stored in project.yml after sealing
- **Delete the manifest after sealing** — Or keep it somewhere very secure
- **Don't keep all bundles together** — That defeats the purpose of splitting
- **Consider printing README.pdf** — Paper backups survive digital disasters

### Rotation

Consider creating a new project every 2-3 years:
- Friends' contact info changes
- You may want to update secrets
- Relationships change
- New cryptographic best practices emerge

You can copy friend configuration:

```bash
rememory init new-project --from old-project
```

## Project Structure

After running all commands, your project looks like:

```
my-recovery-2026/
├── project.yml           # Configuration (friends, threshold, checksums)
├── manifest/             # Your secret files (ADD FILES HERE)
│   ├── README.md         # Default instructions file
│   ├── recovery-codes.txt
│   └── notes.txt
└── output/
    ├── MANIFEST.age      # Encrypted archive of manifest/
    ├── shares/           # Individual share files
    │   ├── SHARE-alice.txt
    │   ├── SHARE-bob.txt
    │   └── ...
    └── bundles/          # Distribution packages
        ├── bundle-alice.zip
        ├── bundle-bob.zip
        └── ...
```

## Commands Reference

| Command | Description |
|---------|-------------|
| `rememory init <name>` | Create a new project |
| `rememory seal` | Encrypt manifest and create shares |
| `rememory bundle` | Generate distribution bundles |
| `rememory status` | Show project status and summary |
| `rememory verify` | Verify integrity of sealed files |
| `rememory verify-bundle <zip>` | Verify a bundle's integrity |
| `rememory recover` | Recover secrets from shares |
| `rememory doc <dir>` | Generate man pages |

For detailed help on any command:

```bash
rememory <command> --help
```
