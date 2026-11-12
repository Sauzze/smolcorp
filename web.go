// Package agentdeck holds what Smolcorp's apps share: the phone,
// pairing and setup pages, and OpenCode's plugin, built into the smolcorp
// command: the Mac app's (inside its bundle), and the Windows and Linux app.
package agentdeck

import "embed"

// Files is Resources/web, Resources/opencode-plugin.js and the briefs that
// come with the app (Resources/briefs). Not Resources/web/office, the sources
// office.js is built from: the apps serve office.js, and only that (see
// scripts/buildoffice).
//
//go:embed Resources/web/*.html Resources/web/*.js Resources/web/*.png Resources/web/sounds
//go:embed Resources/opencode-plugin.js Resources/opencode-plugin-v1.js
//go:embed Resources/briefs/*.md
var Files embed.FS
