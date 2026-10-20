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
or set them aside first.

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
open it, macOS asks them to approve it in System Settings → Privacy & Security
(the README explains how). A Developer ID and Apple's notarisation remove that
step. `release.sh` does both when two environment variables are set. Without
them it signs ad hoc, as it always has.

| Variable | What it does |
|---|---|
| `SMOLCORP_DEVELOPER_ID` | Signs the screen saver, the app and the DMG with this Developer ID Application certificate, with the hardened runtime on and a secure timestamp. The entitlements are in `packaging/macos/Smolcorp.entitlements`: Apple Events (for Terminal, Spotify and Music) and the mic (for dictation). |
| `SMOLCORP_NOTARY_PROFILE` | Sends the DMG to Apple's notary service and waits for it, then staples the ticket to the DMG and to the app before zipping it, so both open without a warning, even offline. Needs `SMOLCORP_DEVELOPER_ID`. |

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

```bash
SMOLCORP_DEVELOPER_ID="Developer ID Application: Your Name (AB12CD34EF)" \
SMOLCORP_NOTARY_PROFILE=smolcorp-notary \
scripts/release.sh
```

Notarisation usually takes a few minutes. If Apple rejects it, the script
stops. See why with:

```bash
xcrun notarytool log <submission-id> --keychain-profile smolcorp-notary
```

Once releases are notarised, the README's install steps can drop the
**Open Anyway** step.
