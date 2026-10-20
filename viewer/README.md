# Portable office viewer (beta)

The office, seen from your phone while you're away: **view only, off by default, end-to-end encrypted**, on a Vercel
project of your own.

Smolcorp on the Mac encrypts a redacted snapshot of the office (AES-256-GCM) and uploads it here every few seconds
while you've turned it on ("Going away: launch portable office"). This folder is a tiny Vercel project: a static
page that decrypts and draws the office in the phone's browser, and four API routes that store the ciphertext in
Upstash Redis for 10 minutes. The server never has the key: it lives in the link's `#k=` fragment, which browsers
never send. This README is the contract between the Mac and the viewer, v1.

No dependencies, no build step: plain files, Node Vercel Functions, and Upstash through its REST API with `fetch`.

```
viewer/
  vercel.json              rewrite /v/:id → /index.html; CSP and headers on every route; no-store on /, /v/*, /api/*
  api/_lib.js              shared: env names, constant-time bearer check, id regex, Redis REST, rate limit, replies
  api/s/[id].js            PUT / GET / DELETE /api/s/:id
  api/s/index.js           DELETE /api/s   (wipe everything)
  api/health.js            GET /api/health (for the setup page's "Check the viewer")
  public/index.html        the page (no inline script or style)
  public/viewer.js         polls, checks freshness, draws read-only
  public/viewer-crypto.js  opens a body; the same file is used by scripts/check-vector.mjs
  public/viewer.css
  public/office.js         a COPY of Resources/web/office.js (run scripts/sync-office.sh after changing that)
  public/robots.txt        disallow everything
  scripts/sync-office.sh   refreshes public/office.js (--check: exit 1 if out of date)
  scripts/check-vector.mjs seals the spec's test vector and opens it with the page's code
  scripts/dev-relay.mjs    local relay on port 7771 (in memory, the real handlers), with a --demo office
```

## Setting it up on Vercel (you do this; nothing is created for you)

1. **Create a Vercel project** on your own account, e.g. `office-viewer`, from your fork of Smolcorp.
   Settings: **Root Directory `viewer/`**, Framework Preset **Other**, **no build command**, output directory left
   as is (`vercel.json` says `public`).
2. **Add Upstash Redis** from the Vercel Marketplace (the free tier is plenty) and connect it to that project.
   It adds `KV_REST_API_URL` and `KV_REST_API_TOKEN` (`UPSTASH_REDIS_REST_URL` / `UPSTASH_REDIS_REST_TOKEN` work too).
3. **Add one environment variable**, `VIEWER_WRITE_SECRET`, for Production only, with the value Smolcorp's setup
   page shows you after "Make a write secret". A value shorter than 32 characters counts as none, and the viewer
   refuses every write until it's set.
4. **Deploy to production.** Paste the production URL (e.g. `https://office-viewer-xxxx.vercel.app`) into the setup
   page's "Viewer address" and press **Check the viewer**: it should say "Viewer ready ✓".
5. *(Optional)* A custom domain. Vercel Authentication can't be used: the phone has no Vercel login.
6. *(Recommended)* **A firewall rule**, so strangers can't use up your Upstash allowance. In the project's
   **Firewall**, add a custom rule: *Request Path starts with `/api/`*, **Rate Limit**, fixed window, 60 s,
   120 requests, keyed by IP, action **Deny**. The relay limits each IP itself too, but each request it counts is a
   Redis command; the firewall stops a flood before it reaches Redis.

What Upstash holds: each office as base64 ciphertext for 10 minutes, under `s:<id>`, and per-minute counters under
`rl:<kind>:<ip>:<minute>` for 2 minutes, so it sees the IP addresses that call the relay. Nothing else, and never a
key. A free Upstash database is plenty for one phone; a phone left polling all day uses a few hundred thousand
commands a month.

Then scan the setup page's QR code with your phone and keep the link (bookmark or Home Screen). It looks like
`https://…/v/<id>#k=<key>`. **Rotate key** on the setup page if you ever lose the phone or share the link by mistake.

## The relay protocol ("bring your own viewer")

The relay is four routes over any key-value store with expiry. Anything that implements this table (a Cloudflare
Worker with KV, a tiny server of your own) works with Smolcorp and with `public/` unchanged.

| Method & path | Auth | Does | Replies |
|---|---|---|---|
| `PUT /api/s/:id` | `Authorization: Bearer <write secret>` | store the raw body under `s:<id>` for **600 s** | `204`; `400` bad id, empty body, or first byte not `0x01`; `401`; `413` over 256 KiB |
| `GET /api/s/:id` | none | return the stored body, `application/octet-stream` | `200`; `404` when absent, expired, or the id is malformed (same reply, a malformed id without a storage lookup) |
| `DELETE /api/s/:id` | Bearer | delete it | `204` (also when absent); `401` |
| `DELETE /api/s` | Bearer | delete **every** stored body | `204`; `401` |
| `GET /api/health` *(optional)* | Bearer | check configuration and storage | `204`; `401`; `503` with a short reason |

- `:id` is 22 characters of base64url, `^[A-Za-z0-9_-]{22}$`.
- The bearer is compared in constant time. With no write secret configured, or one shorter than 32 characters,
  every write is `503`, never allowed.
- Other methods: `405`. No CORS (the page and the API share an origin). Rate limit: 60 GETs and 30 writes per
  minute per IP, then `429`. The bearer is checked first: only authorised writes count toward the 30, and refused
  writes have their own bucket of 10 per minute per IP, so a stranger on the same IP can't use up the Mac's. Every
  reply: `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`,
  `X-Robots-Tag: noindex, nofollow`.
- Bodies are opaque to the relay: `0x01 ‖ nonce(12) ‖ AES-256-GCM ciphertext ‖ tag(16)`, with AAD
  `smolcorp-viewer:v1:<id>`. The relay must not log them.
- The page is served at `/v/:id` (the same `index.html` for every id) with the CSP below, and `/` shows "Not active".

Headers on every route (`vercel.json`):

```
Content-Security-Policy: default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; manifest-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
Referrer-Policy: no-referrer
X-Robots-Tag: noindex, nofollow
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Cross-Origin-Opener-Policy: same-origin
Cross-Origin-Resource-Policy: same-origin
Permissions-Policy: (every feature) =()
Cache-Control: no-store            (on /, /index.html, /v/*, /api/*)
```

The Home Screen icons (`/icon-v3-*.png`, and `/apple-touch-icon*.png` for older lookups) and
`/manifest.webmanifest` get `Cross-Origin-Resource-Policy: cross-origin` and a day's cache instead, as iOS fetches
them outside the page. `index.html` also carries the 180 one inline, as a `data:` URL: iOS's own fetch of linked
icons can fail on a phone's network, and then the Home Screen gets the title's first letter, an "O". The manifest has no `start_url`, `scope` or `id` on purpose: the app then starts at the page
it was added from, `/v/<id>#k=<key>`, and a fixed start URL would lose the link.

## The page

- Reads the id from `/v/<id>` and the key from `#k=<key>`; anything malformed is "Not active".
- Polls `GET /api/s/<id>` every 5 s while visible (paused when hidden; 30 s after 3 misses in a row).
- Decrypts in the browser (non-extractable key), then checks `v === 1`, that `at` is newer than the last shown
  (an older body is ignored), and that it's at most 10 minutes old (±2 min for clock skew), else "Not active".
  The newest `at` shown is kept for each id in the browser's storage, a number and nothing else, so a reload
  can't be shown an older body again.
- Any v2 changes the AAD's `v1` text as well as the version byte: the byte isn't covered by GCM, the AAD is.
- Draws with `AgentOffice.create(canvas, {})`: no handlers, `pointer-events: none`, taps and keys stopped before
  `office.js` hears them, the room's sound forced off. The only request it makes is the GET.
- "Live · 12 s ago" in the corner while live. On Stop (404), expiry, a wrong key or a tampered body: the canvas is
  wiped and only "Not active" is shown.

## Trying it locally

```
node viewer/scripts/check-vector.mjs                    # the spec's test vector, opened with the page's code
node viewer/scripts/dev-relay.mjs --demo                # http://127.0.0.1:7771, prints the demo link
node viewer/scripts/dev-relay.mjs --demo --ttl 20       # expiry in 20 s instead of 600 (dev relay only)
node viewer/scripts/dev-relay.mjs --seal --id <id> [--age 900] [--hour 22] > body.bin   # a sealed sample body
viewer/scripts/sync-office.sh                           # after changing Resources/web/office.js
```

The dev relay serves `public/` with `vercel.json`'s rewrite and headers, and runs the **real** `api/` handlers over
an in-memory Redis. It listens on 127.0.0.1 only. Its write secret is `dev-write-secret-for-this-mac-only` (or `--secret …`), and the
demo uses a fixed, fake id and key. `--demo` seals a sample office of four projects every 5 s. A DELETE of the demo
id (or `DELETE /api/s`) pauses the demo for 2 minutes, as the Mac's Stop would.
