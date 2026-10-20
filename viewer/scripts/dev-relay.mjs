// A local relay for trying the viewer: the real api/ handlers over an in-memory
// Redis, viewer/public served with vercel.json's rewrite and headers. Localhost only.
//
//   node viewer/scripts/dev-relay.mjs [--demo] [--port 7771] [--ttl 600] [--secret dev-write-secret-for-this-mac-only]
//   node viewer/scripts/dev-relay.mjs --seal [--id <id>] [--age <seconds old>] [--hour <H>] > body.bin
//     (a demo body, sealed with the dev key; PUT it yourself)
//
// --demo seals a sample office every 5 s, with a fixed dev id and key (never real ones), and prints its link.
// A DELETE of the demo id (or of everything) pauses the demo for 2 minutes, as the Mac's Stop would.
// --ttl changes the 10-minute expiry, for this dev relay only.
import http from 'node:http';
import { readFileSync, existsSync, statSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join, normalize, extname, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const viewer = join(here, '..');
const pub = join(viewer, 'public');
const require = createRequire(import.meta.url);

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const option = (name, fallback) => { const i = args.indexOf(name); return i >= 0 && i + 1 < args.length ? args[i + 1] : fallback; };

const PORT = Number(option('--port', 7771));
const TTL = Number(option('--ttl', 600));
const SECRET = option('--secret', 'dev-write-secret-for-this-mac-only');

/* ------------------------------------------------- the dev id and key */

const b64url = (bytes) => Buffer.from(bytes).toString('base64url');
// Fixed and obviously fake, so screenshots and docs can show them.
const DEMO_ID = b64url(Buffer.from('smolcorp-demo-01'));                  // 16 bytes → 22 chars
const DEMO_KEY = b64url(Buffer.from('smolcorp-viewer-dev-key-not-real')); // 32 bytes → 43 chars

async function seal(payload, id, keyText) {
  const json = JSON.stringify(payload);
  const size = Math.ceil(Buffer.byteLength(json) / 1024) * 1024 || 1024;
  const plain = Buffer.from(json.padEnd(json.length + size - Buffer.byteLength(json), ' '));
  const key = await crypto.subtle.importKey('raw', Buffer.from(keyText, 'base64url'), 'AES-GCM', false, ['encrypt']);
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const aad = new TextEncoder().encode(`smolcorp-viewer:v1:${id}`);
  const sealed = new Uint8Array(await crypto.subtle.encrypt({ name: 'AES-GCM', iv: nonce, additionalData: aad, tagLength: 128 }, key, plain));
  return Buffer.concat([Buffer.from([0x01]), nonce, sealed]);
}

/* ------------------------------------------------- the sample office */

// Four of the sandbox's sample projects, redacted as spec §3 says (aliases, no
// activity, tool, model or anything to press).
const STARTED = Date.now();
function sampleOffice(step) {
  const now = Date.now();
  const desk = (n, name, agent, agentName, state, extra = {}) => ({
    pin: null,
    session: {
      id: `v${n}`, name, project: name, state, effort: '', pending: false, agent, agentName,
      compacting: false, turnStartedAt: null, errorKind: null, subagents: [], role: null,
      dial: false, mic: false, dictates: false, keys: false, fire: false, summary: false, ...extra,
    },
  });
  const docs = ['unread', 'idle', 'thinking'][Math.floor(step / 3) % 3];
  return {
    keys: [
      desk(1, 'api-server', 'claude', 'Claude Code', step % 6 < 3 ? 'asking' : 'thinking', { effort: 'high', turnStartedAt: STARTED - 12 * 60000 }),
      desk(2, 'web-app', 'codex', 'Codex', 'thinking', { turnStartedAt: STARTED - 95000 }),
      desk(3, 'docs', 'gemini', 'Gemini CLI', docs, docs === 'thinking' ? { turnStartedAt: now - 20000 } : {}),
      desk(4, 'billing', 'claude', 'Claude Code', 'thinking', {
        effort: 'medium', turnStartedAt: STARTED - 240000,
        subagents: [1, 2, 3].map((i) => ({ id: `v4.${i}`, type: 'Explore', model: null })),
      }),
    ],
    overflow: 0,
    now,
    theme: {},
    music: { playing: true },
    scroll: false,
    dictation: null,
    hiring: { fire: false, hire: false, roles: [] },
    layout: { keys: 'auto', office: true, dial: true, stick: true, room: {}, rows: [] },
  };
}

if (flag('--seal')) {
  const age = Number(option('--age', 0));
  // --hour H: as if the Mac's clock said H:00 now, shifted as sandbox/tools/shoot.mjs shifts the phone's, for night shots.
  const d = new Date();
  const shift = args.includes('--hour') ? ((Number(option('--hour', 0)) - d.getHours()) * 60 - d.getMinutes()) * 60000 : 0;
  const at = Date.now() + shift - age * 1000;
  const body = await seal({ v: 1, at, office: { ...sampleOffice(0), now: at } }, option('--id', DEMO_ID), DEMO_KEY);
  process.stdout.write(body);
  process.exit(0);
}

/* ------------------------------------------------- Redis, in memory */

const store = new Map(); // key → { value, expires (ms) | null }
const alive = (k) => {
  const e = store.get(k);
  if (e && e.expires !== null && e.expires <= Date.now()) { store.delete(k); return null; }
  return e || null;
};
const glob = (pattern) => new RegExp(`^${pattern.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.')}$`);

async function memoryRedis(commands) {
  return commands.map(([name, ...a]) => {
    switch (String(name).toUpperCase()) {
      case 'PING': return 'PONG';
      case 'GET': { const e = alive(a[0]); return e ? e.value : null; }
      case 'SET': {
        const i = a.findIndex((x) => String(x).toUpperCase() === 'EX');
        store.set(a[0], { value: String(a[1]), expires: i >= 0 ? Date.now() + Number(a[i + 1]) * 1000 : null });
        return 'OK';
      }
      case 'DEL': return a.filter((k) => alive(k) && store.delete(k)).length;
      case 'INCR': {
        const e = alive(a[0]);
        const value = String((e ? Number(e.value) : 0) + 1);
        store.set(a[0], { value, expires: e ? e.expires : null });
        return Number(value);
      }
      case 'EXPIRE': { const e = alive(a[0]); if (!e) return 0; e.expires = Date.now() + Number(a[1]) * 1000; return 1; }
      case 'SCAN': {
        const i = a.findIndex((x) => String(x).toUpperCase() === 'MATCH');
        const match = glob(i >= 0 ? String(a[i + 1]) : '*');
        return ['0', [...store.keys()].filter((k) => alive(k) && match.test(k))];
      }
      default: throw new Error('unknown command');
    }
  });
}

const lib = require('../api/_lib.js');
lib.configure({ redis: memoryRedis, ttl: TTL, secret: () => SECRET });
const routes = {
  item: require('../api/s/[id].js'),
  all: require('../api/s/index.js'),
  health: require('../api/health.js'),
};

/* ------------------------------------------------- vercel.json */

const vercel = JSON.parse(readFileSync(join(viewer, 'vercel.json'), 'utf8'));
// Enough of Vercel's source patterns for this file: literal paths, "(.*)" and ":name".
const pattern = (source) => new RegExp(`^${source.replace(/:[a-z]+/gi, '[^/]+')}$`);
const headerRules = vercel.headers.map((rule) => ({ test: pattern(rule.source), headers: rule.headers }));
const rewrites = vercel.rewrites.map((rule) => ({ test: pattern(rule.source), to: rule.destination }));

const TYPES = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8', '.png': 'image/png', '.json': 'application/json',
  '.webmanifest': 'application/manifest+json; charset=utf-8' };

function serveStatic(req, res, path) {
  if (req.method !== 'GET' && req.method !== 'HEAD') { res.setHeader('Allow', 'GET, HEAD'); res.statusCode = 405; return res.end('method not allowed'); }
  const rewrite = rewrites.find((r) => r.test.test(path));
  let file = rewrite ? rewrite.to : path === '/' ? '/index.html' : path;
  let full;
  try { full = normalize(join(pub, decodeURIComponent(file))); } catch { full = ''; }
  if (!full.startsWith(pub + sep) || !existsSync(full) || !statSync(full).isFile()) {
    res.statusCode = 404;
    res.setHeader('Content-Type', 'text/plain; charset=utf-8');
    return res.end('not found');
  }
  const data = readFileSync(full);
  res.setHeader('Content-Type', TYPES[extname(full)] || 'application/octet-stream');
  res.setHeader('Content-Length', String(data.length));
  res.statusCode = 200;
  res.end(req.method === 'HEAD' ? undefined : data);
}

/* ------------------------------------------------- the server */

let demoPausedUntil = 0;

const server = http.createServer(async (req, res) => {
  const path = (req.url || '/').split('?')[0];
  // As on Vercel, the client can't choose the IP the rate limit counts.
  delete req.headers['x-real-ip'];
  delete req.headers['x-forwarded-for'];
  for (const rule of headerRules) if (rule.test.test(path)) for (const h of rule.headers) res.setHeader(h.key, h.value);
  try {
    if (path === '/api/health') return await routes.health(req, res);
    if (path === '/api/s') {
      await routes.all(req, res);
      if (req.method === 'DELETE' && res.statusCode === 204) demoPausedUntil = Date.now() + 120000;
      return;
    }
    if (/^\/api\/s\/[^/]+$/.test(path)) {
      await routes.item(req, res);
      if (req.method === 'DELETE' && res.statusCode === 204 && path.endsWith(`/${DEMO_ID}`)) demoPausedUntil = Date.now() + 120000;
      return;
    }
    if (path.startsWith('/api/')) { res.statusCode = 404; res.setHeader('Content-Type', 'text/plain; charset=utf-8'); return res.end('not found'); }
    return serveStatic(req, res, path);
  } catch {
    if (!res.headersSent) res.statusCode = 500;
    res.end();
  }
});

server.listen(PORT, '127.0.0.1', () => {
  console.log(`dev relay on http://127.0.0.1:${PORT}  (ttl ${TTL} s, write secret: ${SECRET === 'dev-write-secret-for-this-mac-only' ? 'dev-write-secret-for-this-mac-only' : 'from --secret'})`);
  if (flag('--demo')) console.log(`demo office: http://localhost:${PORT}/v/${DEMO_ID}#k=${DEMO_KEY}`);
});

if (flag('--demo')) {
  let step = 0;
  const beat = async () => {
    if (Date.now() < demoPausedUntil) return;
    const body = await seal({ v: 1, at: Date.now(), office: sampleOffice(step++) }, DEMO_ID, DEMO_KEY);
    await memoryRedis([['SET', `s:${DEMO_ID}`, body.toString('base64'), 'EX', TTL]]);
  };
  beat();
  setInterval(beat, 5000);
}
