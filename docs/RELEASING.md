# Releasing Smolcorp for macOS

```bash
scripts/release.sh
```

This builds one universal app (Apple silicon and Intel) from what's committed,
and makes two downloads from it, printing each one's size and SHA-256 for the
release notes:

| File | What it's for |
|---|---|
| `build/Smolcorp.dmg` | The download for most people: open it, drag Smolcorp to Applications. |
| `build/Smolcorp.zip` | The same app, zipped, for the Homebrew cask and older links. Keep uploading it. |

The script won't run with uncommitted changes or untracked files: commit them
or set them aside first. It needs [Go](https://go.dev) 1.26 or later, as any
build of the app does: the app's core, `Contents/Resources/bin/smolcorp`, is
built from `cmd/smolcorp` for both architectures.

## The DMG

`scripts/make-dmg.sh` makes the DMG from `build/Smolcorp.app`, so after
`scripts/build-app.sh universal` you can run it on its own. It uses
[dmgbuild](https://github.com/dmgbuild/dmgbuild), which writes the window's
layout itself, so nothing opens on screen and neither Finder nor AppleScript
is involved.

The first time, it sets up a Python venv at `build/dmg-venv` with dmgbuild in
it. That needs Python 3 (the Xcode Command Line Tools have it) and the
internet, once. Nothing is installed outside `build/`, which git ignores.
Delete `build/dmg-venv` to undo it.

The window is in `packaging/macos/dmg/`:

- `settings.py`: the window's size, where the icons go, the volume's name and icon.
- `background.png` and `background@2x.png`: a corner of the office, in the
  office's pixel art and palette, with the arrow and the line "Drag Smolcorp to
  Applications". `make-background.py` draws them. Run it again after changing
  the art or moving the icons.
- `preview.py`: shows how the window will look, with the icons where
  `settings.py` puts them, without opening Finder.

Both Python scripts need Pillow in the venv:

```bash
build/dmg-venv/bin/pip install pillow
build/dmg-venv/bin/python packaging/macos/dmg/make-background.py
build/dmg-venv/bin/python packaging/macos/dmg/preview.py build/dmg-preview.png
```

## Signing and notarisation

By default the app is **signed ad hoc**: it works, but the first time people
open it, macOS asks them to approve it in System Settings → Privacy & Security.
A Developer ID and Apple's notarisation remove that
step. `release.sh` does both when two environment variables are set. Without
them it signs ad hoc, as it always has.

| Variable | What it does |
|---|---|
| `SMOLCORP_DEVELOPER_ID` | Signs the screen saver, the app and the DMG with this Developer ID Application certificate, with the hardened runtime on and a secure timestamp. The entitlements are in `packaging/macos/Smolcorp.entitlements`: Apple Events (for Terminal, Spotify and Music) and the mic (for dictation). |
| `SMOLCORP_NOTARY_PROFILE` | Sends the DMG to Apple's notary service and waits for it, staples the ticket to the app, then makes the DMG again around it and has that notarised and stapled too (two submissions), so the DMG, the app in it and the zipped app each carry a ticket. Needs `SMOLCORP_DEVELOPER_ID`. |

### Setting it up once

You need a paid membership in the Apple Developer Program on your own Apple
Account. The certificate and the notary profile below belong to that account
and stay in your login keychain. Nothing about them goes in this repository.

1. **Join the Apple Developer Program** at
   [developer.apple.com/programs](https://developer.apple.com/programs/) with
   your Apple Account, as an individual. Your name, as Apple has it, is what
   macOS shows as the developer.
2. **Make a Developer ID Application certificate.** In Xcode, open
   **Settings → Accounts**, add your Apple Account, select your team, click
   **Manage Certificates…**, then **+ → Developer ID Application**. The
   certificate and its private key go into your login keychain. Check that it's
   there:

   ```bash
   security find-identity -v -p codesigning
   ```

   You'll see a line like
   `"Developer ID Application: Your Name (AB12CD34EF)"`. That whole quoted
   name is your `SMOLCORP_DEVELOPER_ID`. The ten characters in brackets are
   your Team ID.
3. **Make an app-specific password** for the notary service at
   [account.apple.com](https://account.apple.com) → **Sign-In and Security →
   App-Specific Passwords**.
4. **Save a notary profile in your keychain**, so the password never sits in
   a script or a file:

   ```bash
   xcrun notarytool store-credentials smolcorp-notary \
     --apple-id "you@example.com" --team-id AB12CD34EF
   ```

   It asks for the app-specific password. `smolcorp-notary` is the profile's
   name, your `SMOLCORP_NOTARY_PROFILE`.

### Releasing with them

First, a dry run: it says which identity and notary profile it would use and
whether this Mac has the identity, and signs, builds and sends nothing.

```bash
SMOLCORP_DEVELOPER_ID="Developer ID Application: Your Name (AB12CD34EF)" \
SMOLCORP_NOTARY_PROFILE=smolcorp-notary \
scripts/release.sh --check-signing
```

Then the release itself:

```bash
SMOLCORP_DEVELOPER_ID="Developer ID Application: Your Name (AB12CD34EF)" \
SMOLCORP_NOTARY_PROFILE=smolcorp-notary \
scripts/release.sh
```

Notarisation usually takes a few minutes each time. If Apple rejects it, the
script stops. See why with:

```bash
xcrun notarytool log <submission-id> --keychain-profile smolcorp-notary
```

Releases are notarised since 1.24.2, so the README's and FIRST-SETUP.md's
install steps have no **Open Anyway** step. A release made without
`SMOLCORP_NOTARY_PROFILE` would need it back.

## Every build is hardened, and who may sign it

Every Mac build has the hardened runtime: a release, a copy of your own built by
`scripts/build-app.sh` or `install.sh`, and a test copy. That covers the app, the
`smolcorp` command and tmux inside it. Without it, any program of yours (a hire
among them) could attach a debugger to them, or slip a library in with
`DYLD_INSERT_LIBRARIES`, and read the keys they hold in memory, or change the
guard's answer. `scripts/check-runtime.sh` checks each file after signing, and a
build without the runtime stops there. `scripts/build-app.sh --check-signing`
says how it would sign, and signs and builds nothing.

Smolcorp's Keychain items open for any app signed with the same certificate as
Smolcorp. So a program of yours that can sign with your "Smolcorp Local" or
Developer ID identity could build a changed Smolcorp that reads them. Three
identities keep that apart:

| Identity | Signs | Where it lives |
|---|---|---|
| Developer ID | releases | a keychain of its own, locked when not signing |
| "Smolcorp Local" | the copy you install yourself | the same keychain, or one like it |
| "Smolcorp Test" | the team's test copies, named in `SMOLCORP_TEST_SIGN_IDENTITY` (ad hoc without it) | the testers' own keychains: its requirement isn't the app's, so a test copy never reads the installed app's items |

To move your identities into a keychain of their own, once (your step; nothing
in this repository does it for you):

```bash
security create-keychain ~/Library/Keychains/smolcorp-signing.keychain-db   # asks for a new password
security set-keychain-settings -l -u -t 300 ~/Library/Keychains/smolcorp-signing.keychain-db  # locks on sleep and after 5 minutes
```

Then, in Keychain Access, move the identities (certificate and private key)
from the login keychain into it. A private key moved like that usually lets
`codesign` in only after a "wants to access key" dialog, at every build. To let
it in once and for all, give the key's partition list `codesign` (it asks for
the keychain's password):

```bash
security set-key-partition-list -S apple-tool:,apple: -s ~/Library/Keychains/smolcorp-signing.keychain-db
```

Point the build at it:

```bash
SMOLCORP_SIGN_KEYCHAIN=~/Library/Keychains/smolcorp-signing.keychain-db scripts/build-app.sh
```

It asks for that keychain's password in the terminal, signs, and locks the
keychain again when it ends, however it ends. It refuses to name the login
keychain, which it would otherwise lock. If the identity isn't in that keychain,
it signs ad hoc and says so. A test copy is never signed with these
identities, so building one asks for nothing. `scripts/release.sh` doesn't take
`SMOLCORP_SIGN_KEYCHAIN` yet: with the Developer ID in that keychain, macOS asks
for its password when the release signs, and you lock it again yourself
(`security lock-keychain <file>`).
