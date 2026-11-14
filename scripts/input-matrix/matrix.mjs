// What the phone types and says reaches every kind of desk, byte for byte, or the phone is told
// why not. Real sessions, each on a tmux server of its own, reporting to a test copy; the text
// goes the way the deck page sends it (POST /dictate, as through `tailscale serve`), and what
// lands is read back from the desk itself: its input box (capture-pane), its transcript, or the
// stand-in agent's byte log.
//
//   node scripts/input-matrix/matrix.mjs up    # starts the desks, writes desks.json
//   node scripts/input-matrix/matrix.mjs run   # the matrix, as a Markdown table on stdout (MATRIX_KEEP=1: without the endings)
//   node scripts/input-matrix/matrix.mjs window hire  # the same text with a client attached, as a window is
//   node scripts/input-matrix/matrix.mjs end   # only the endings: each desk's agent or tmux gone, last
//   node scripts/input-matrix/matrix.mjs down  # ends every desk it started, and their servers
//
// Settings, from the environment:
//   MATRIX_PORT     the test copy's port (7780)
//   MATRIX_SUPPORT  its support folder (build/test-support): its sessions.json, and by default its hooks
//   MATRIX_CLAUDE_SETTINGS  the --settings file whose hooks report to the app under test
//                   ($MATRIX_SUPPORT/claude-settings.json, which scripts/test-copy.sh writes)
//   MATRIX_HOOK     the command line the stand-in agent reports through, "codex" added after it and
//                   the event on its stdin ($MATRIX_SUPPORT/report.sh, the same); for Go parity, the
//                   Go core's own: '/path/to/smolcorp' hook --port <its port>
//   MATRIX_KEY      a file holding a paired phone's key (pair one in the copy's setup first)
//   MATRIX_DIR      where desks.json, the stand-in's log and the canaries go (a scratch folder)
//   MATRIX_SOCKETS  the prefix of the tmux sockets it makes (i1): only those are ever touched
//   MATRIX_MODEL    the Claude model of the real sessions (haiku)
//   MATRIX_GM_TOKEN a file holding the copy's GM token: the GM desk then signs one call as the GM,
//                   as a GM started by hand does (with "Allow a GM started by hand" on)
//   MATRIX_ONLY     comma-separated desk names to run (all)
//   MATRIX_LAN      the copy's address on the Wi-Fi, as a phone there opens it (http://192.168.1.4:7780):
//                   the plain-LAN rows (I34) need it, and say SKIPPED without
//   MATRIX_LAUNCHER Go engine only (I5): the command line that starts an agent in a launcher's window, as
//                   on Windows and Linux ('SMOLCORP_PORT=7781 /path/to/smolcorp' run): a "launcher" desk
//                   then runs the stand-in through it, and every stand-in row runs there too
//
// The desks: gm (Claude, GM role), mgr (Claude, a manager started by hand in a shell),
// hire (Claude in a hire's tmux session) and fake (a non-Claude agent: fake-agent.mjs, as Codex); on the Go
// engine, launcher too (the stand-in in a launcher's window, with MATRIX_LAUNCHER).
// T3 Code desks and taken-over hires are PENDING until their lanes merge: their rows say what
// each must show. A taken-over hire still in tmux runs as any desk here once `up` makes one; a
// T3 desk has no pane to read, so its text is read back with `t3 thread show <id> --json`.
//
// Every Claude session runs with --permission-mode default, so nothing it might be asked runs
// without a prompt, and with --setting-sources project,local, so your own ~/.claude/settings.json,
// whose hooks report to the Smolcorp you use (7710), isn't loaded: only the app under test hears
// of these desks. Costs: four short turns per Claude desk (short, long, the prompt, the follow-up).
import { execFileSync, spawn, spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, '../..');
const PORT = process.env.MATRIX_PORT || '7780';
const SUPPORT = process.env.MATRIX_SUPPORT || join(repo, 'build/test-support');
const DIR = process.env.MATRIX_DIR || join(repo, 'build/input-matrix');
const PREFIX = process.env.MATRIX_SOCKETS || 'i1';
const MODEL = process.env.MATRIX_MODEL || 'haiku';
const CLAUDE_SETTINGS = process.env.MATRIX_CLAUDE_SETTINGS || join(SUPPORT, 'claude-settings.json');
const quoted = (word) => `'${word.replace(/'/g, `'\\''`)}'`;
const HOOK = process.env.MATRIX_HOOK || quoted(join(SUPPORT, 'report.sh'));
const KEY = process.env.MATRIX_KEY ? readFileSync(process.env.MATRIX_KEY, 'utf8').trim() : '';
const DESKS = join(DIR, 'desks.json');
const BASE = `http://127.0.0.1:${PORT}`;
const LAN = process.env.MATRIX_LAN || '';
const LAUNCHER = process.env.MATRIX_LAUNCHER || '';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/* ------------------------------------------------------------- tmux, ours only */

function tmux(desk, ...args) {
  if (!desk.socket.startsWith(`${PREFIX}-`)) throw new Error(`not our socket: ${desk.socket}`);
  return spawnSync('tmux', ['-L', desk.socket, ...args], { encoding: 'utf8' });
}
const screen = (desk) => tmux(desk, 'capture-pane', '-p', '-t', desk.target).stdout || '';
const emptyBox = (desk) => tmux(desk, 'send-keys', '-t', desk.target, 'C-e', 'C-u');

/** The input box, as Claude Code draws it ("❯ …", the last such line: those above are past prompts), or the stand-in's. */
function boxLine(desk) {
  const lines = screen(desk).split('\n');
  // Claude Code draws its box's ❯ with a no-break space after it.
  const mark = desk.kind === 'fake' ? /^fake-codex>( |$)/ : /^❯([ \u00a0]|$)/;
  const at = lines.findLastIndex((l) => mark.test(l));
  return at < 0 ? null : lines[at].replace(mark, '').replace(/\s+$/, '');
}

/* ------------------------------------------------------------- the copy */

/** The session's state, as the phone is told it. */
async function snapshot() {
  const res = await fetch(`${BASE}/events?t=${encodeURIComponent(KEY)}`, { signal: AbortSignal.timeout(3000) });
  const reader = res.body.getReader();
  let text = '';
  while (!text.includes('\n\n')) {
    const { value, done } = await reader.read();
    if (done) break;
    text += Buffer.from(value).toString('utf8');
  }
  reader.cancel().catch(() => {});
  const data = JSON.parse(text.split('\n').find((l) => l.startsWith('data:')).slice(5));
  const all = [...data.keys.map((k) => k.session).filter(Boolean), ...(data.overflow || [])];
  return new Map(all.map((s) => [s.id, s]));
}
async function stateOf(id) { return (await snapshot()).get(id)?.state ?? 'gone'; }
async function waitState(id, want, ms = 60000) {
  const until = Date.now() + ms;
  while (Date.now() < until) {
    const state = await stateOf(id);
    if (want.includes(state)) return state;
    await sleep(700);
  }
  return stateOf(id);
}

/** A turn: begun (not idle any more), then over. */
async function turn(id) {
  await waitState(id, ['thinking', 'asking', 'unread'], 15000);
  return waitState(id, ['unread', 'idle', 'asking'], 90000);
}

/**
 * POST /dictate as the deck sends it over Tailscale's HTTPS: from this computer's loopback, passed on by a proxy.
 * `lan`: as a phone on the Wi-Fi sends it, over plain http to the copy's LAN address (I34), with `key` (the paired one).
 */
async function dictate(id, text, send, { lan = false, key = KEY, path = '/dictate' } = {}) {
  const where = lan ? LAN : BASE;
  const headers = lan ? { 'Content-Type': 'application/json' }
    : { 'Content-Type': 'application/json', 'X-Forwarded-For': '100.101.102.103', 'Tailscale-User-Login': 'tester@example.com' };
  const res = await fetch(`${where}${path}?${key === null ? '' : `t=${encodeURIComponent(key)}&`}session=${encodeURIComponent(id)}`, {
    method: 'POST', headers, body: JSON.stringify({ text, send }),
  });
  return { status: res.status, body: await res.text() };
}

/**
 * As PhoneDictation.prepared: line breaks to line feeds, other control characters but a tab to spaces, trimmed;
 * of several lines, the lines as they are (I34); of one, a tab a space too and runs of spaces one.
 */
const prepared = (text) => {
  // Swift's .whitespacesAndNewlines, not JS's \s, which trims U+FEFF but neither U+0085 nor U+200B.
  const space = '[\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200b\\u2028\\u2029\\u202f\\u205f\\u3000]';
  const lines = text.replace(/\r\n/g, '\n').replace(/[\r\u0085\u2028\u2029]/g, '\n')
    .replace(/[\u0000-\u0008\u000b-\u001f\u007f-\u009f]/g, ' ').replace(new RegExp(`^${space}+|${space}+$`, 'g'), '');
  return lines.includes('\n') ? lines : lines.replace(/\t/g, ' ').split(' ').filter(Boolean).join(' ');
};

/** The session's last prompt, from its transcript (Claude Code) or the stand-in's log. */
function lastPrompt(desk) {
  if (desk.kind === 'fake') {
    const rows = logRows(desk).filter((r) => 'submit' in r);
    return rows.at(-1)?.submit ?? null;
  }
  const transcript = sessionsFile().find((s) => s.id === desk.id)?.transcript;
  if (!transcript || !existsSync(transcript)) return null;
  const prompts = readFileSync(transcript, 'utf8').trim().split('\n').map((l) => { try { return JSON.parse(l); } catch { return {}; } })
    .filter((e) => e.type === 'user' && typeof e.message?.content === 'string');
  const prompt = prompts.at(-1)?.message.content ?? null;
  // A long paste is kept in the transcript between Claude Code's own tags, on lines of their own.
  const pasted = prompt?.match(/^\s*<pasted_content id="[^"]*">\n([\s\S]*)\n<\/pasted_content id="[^"]*">\s*$/);
  return pasted ? pasted[1] : prompt;
}
/** The stand-in's bytes since `from` (an index into its log), as text. */
const logRows = (desk) => readFileSync(desk.log, 'utf8').split('\n').filter(Boolean).map((l) => JSON.parse(l));
function fakeBytes(desk, from) {
  return Buffer.concat(logRows(desk).slice(from).filter((r) => r.bytes).map((r) => Buffer.from(r.bytes, 'hex'))).toString('utf8');
}
const fakeMark = (desk) => logRows(desk).length;
const sessionsFile = () => JSON.parse(readFileSync(join(SUPPORT, 'sessions.json'), 'utf8')).sessions;

/* ------------------------------------------------------------- up and down */

async function up() {
  mkdirSync(DIR, { recursive: true });
  if (!existsSync(CLAUDE_SETTINGS)) throw new Error(`no ${CLAUDE_SETTINGS}: start the copy first, or set MATRIX_CLAUDE_SETTINGS`);
  // Only the app under test's hooks: never the user's settings, which report to the Smolcorp in use.
  const claude = `claude --model ${MODEL} --permission-mode default --setting-sources project,local --settings ${quoted(CLAUDE_SETTINGS)}`;
  const plan = [
    { name: 'gm', kind: 'claude', role: 'gm', socket: `${PREFIX}-gm`, target: 'gm', command: claude },
    // A manager started by hand: typed into a shell, which stays when the agent ends.
    { name: 'mgr', kind: 'claude', socket: `${PREFIX}-mgr`, target: 'mgr', typed: claude },
    // As a hire's own server: one session called hire, the agent its only process.
    { name: 'hire', kind: 'claude', socket: `${PREFIX}-hire`, target: 'hire', command: claude },
    // Typed into a shell too, so it can be killed with its terminal left open.
    { name: 'fake', kind: 'fake', socket: `${PREFIX}-fake`, target: 'hire', log: join(DIR, 'fake.log'),
      typed: `(exec -a codex ${process.execPath} ${join(here, 'fake-agent.mjs')} ${join(DIR, 'fake.log')} ${quoted(HOOK)})` },
  ];
  // Go engine (I5): the stand-in in a launcher's window, as `smolcorp run codex` starts an agent on Windows and
  // Linux; "codex" a script of the matrix's own that runs the stand-in. Its session's tab is the launcher's.
  if (LAUNCHER) {
    const bin = join(DIR, 'bin'), log = join(DIR, 'launcher.log');
    mkdirSync(bin, { recursive: true });
    writeFileSync(join(bin, 'codex'), `#!/bin/sh\nexec ${quoted(process.execPath)} ${quoted(join(here, 'fake-agent.mjs'))} ${quoted(log)} ${quoted(HOOK)}\n`, { mode: 0o700 });
    plan.push({ name: 'launcher', kind: 'fake', launcher: true, socket: `${PREFIX}-launcher`, target: 'launcher', log,
      typed: `PATH=${quoted(bin)}:"$PATH" ${LAUNCHER} codex` });
  }
  // Sessions already known: a new one in the same tab may take an old one's start time, so it's told by its id.
  const before = new Set(existsSync(join(SUPPORT, 'sessions.json')) ? sessionsFile().map((s) => s.id) : []);
  const desks = [];
  for (const d of plan) {
    // Inside this repo, which Claude Code already trusts, each in a folder of its own.
    const folder = join(repo, 'build/input-matrix-desks', d.name);
    mkdirSync(folder, { recursive: true });
    if (d.log) writeFileSync(d.log, '');
    // A plain interactive bash, with no history written: the agent is its job in front, as when you type it.
    const run = d.command ? ['/bin/bash', '-c', d.command] : ['/bin/bash', '--norc', '--noprofile', '-i'];
    tmux(d, 'new-session', '-d', '-s', d.target, '-x', '320', '-y', '45', '-c', folder, '-e', 'HISTFILE=/dev/null', ...run);
    if (d.typed) tmux(d, 'send-keys', '-t', d.target, '-l', '--', d.typed), tmux(d, 'send-keys', '-t', d.target, 'Enter');
    const socketPath = tmux(d, 'display-message', '-p', '#{socket_path}').stdout.trim();
    const pane = tmux(d, 'display-message', '-p', '-t', d.target, '#{pane_id}').stdout.trim();
    desks.push({ name: d.name, kind: d.kind, role: d.role, socket: d.socket, target: d.target, log: d.log, launcher: d.launcher, tab: `tmux:${pane}@${socketPath}` });
  }
  // Each desk's session, by its tab, once it has reported in: the one started now, not one an earlier run left.
  for (let i = 0; i < 40 && desks.some((d) => !d.id); i++) {
    await sleep(1000);
    for (const d of desks) {
      d.id ||= sessionsFile().filter((s) => (d.launcher ? s.tab?.startsWith('launcher:') : s.tab === d.tab) && !before.has(s.id)).at(-1)?.id;
    }
  }
  const gm = desks.find((d) => d.role === 'gm');
  if (gm?.id && process.env.MATRIX_GM_TOKEN) {
    const res = await fetch(`http://localhost:${PORT}/setup/gm/team`, {
      headers: { 'x-agentdeck-setup': '1', 'x-smolcorp-gm-token': readFileSync(process.env.MATRIX_GM_TOKEN, 'utf8').trim(), 'x-smolcorp-session': gm.id },
    });
    gm.gmCall = res.status;
  }
  writeFileSync(DESKS, JSON.stringify(desks, null, 2));
  for (const d of desks) console.log(`${d.name}\t${d.id ?? 'NOT REPORTED'}\t${d.tab}`);
}

function down() {
  const desks = existsSync(DESKS) ? JSON.parse(readFileSync(DESKS, 'utf8')) : [];
  for (const d of desks) {
    const r = tmux(d, 'kill-server');
    console.log(`${d.socket}: ${r.status === 0 ? 'ended' : 'already gone'}`);
  }
}

/* ------------------------------------------------------------- the cases */

const SHORT = 'I1 short check: reply with the single word ok.';
const LONG = ['I1 long check: reply with the single word ok, nothing else.',
  ...Array.from({ length: 20 }, (_, i) => `Ligne ${i + 1} : un texte assez long pour remplir l'écran du téléphone, avec des mots.`)].join('\n');
const FRENCH = 'Café à l’été, ça va ? Où est-ce ? Œuvre côté « guillemets » é è à ç ô œ 🎉👍🏽 👩‍💻 ❤️';
// Relative, so the text stays one line in the box; looked for wherever a shell could have run it.
const canary = (name) => `i1-canary-${name}`;
const ran = (name) => [join(repo, 'build/input-matrix-desks/gm'), join(repo, 'build/input-matrix-desks/mgr'), join(repo, 'build/input-matrix-desks/hire'),
  join(repo, 'build/input-matrix-desks/fake'), repo, DIR, process.env.HOME].some((folder) => existsSync(join(folder, canary(name))));
const SHELLISH = () => `He said "hi" and it's 'ok' $HOME \`touch ${canary('bt')}\` $(touch ${canary('cs')}) ; echo a && echo b \\`;
const SEMI = 'fin de phrase;';
// Several lines, as one bracketed paste (I34): indentation, a tab, a blank line, accents, a trailing ";".
const LINES = 'I34 lines:\n\n    « indentée » 👩‍💻\n\tavec un tab;';
const NO_LINES = "This desk's terminal can't take several lines as one paste, so nothing was typed: send it as one line.";
// "!" alone, and with what makes it one character with the next to Swift while the agent still sees "!" first:
// a combining accent, a zero-width joiner, an emoji variation selector.
const BANGS = [['', 'bang'], ['\u0301', 'bang-301'], ['\u200d', 'bang-200d'], ['\ufe0f', 'bang-fe0f']];
const CANARIES = ['bt', 'cs', 'perm', ...BANGS.map(([, name]) => name)];

/** Types `text` without Return, reads it back from the box, and empties the box. */
async function typedBack(desk, text, how) {
  const mark = desk.kind === 'fake' ? fakeMark(desk) : 0;
  const res = await dictate(desk.id, text, false, how);
  const want = desk.kind === 'fake' ? `\x1b[200~${prepared(text)}\x1b[201~` : prepared(text);
  // Read back until it shows, for up to 3 s: a terminal redrawing has no box for a moment.
  let got = null;
  for (let i = 0; i < 10 && got !== want; i++) {
    await sleep(300);
    got = desk.kind === 'fake' ? fakeBytes(desk, mark) : boxLine(desk);
  }
  emptyBox(desk);
  await sleep(300);
  return { res, got, ok: res.status === 204 && got === want, want };
}

const cases = [
  ['short + Enter', async (desk) => {
    const res = await dictate(desk.id, SHORT, true);
    await turn(desk.id);
    const got = lastPrompt(desk);
    return { pass: res.status === 204 && got === SHORT, note: `${res.status}; prompt ${got === SHORT ? 'exact' : JSON.stringify(got)}` };
  }],
  ['long multi-line + Enter', async (desk) => {
    const res = await dictate(desk.id, LONG, true);
    await turn(desk.id);
    const got = lastPrompt(desk);
    return { pass: res.status === 204 && got === LONG, note: `${res.status}; ${LONG.length} chars, ${LONG.split('\n').length} lines; prompt ${got === LONG ? 'exact, every line (one paste)' : JSON.stringify(got?.slice(0, 80))}` };
  }],
  // I34: several lines typed as ONE bracketed paste, byte for byte; read from the stand-in's bytes (a Claude box
  // shows a long paste folded: its exact lines are the "long multi-line" row's transcript).
  ['multi-line typed: one bracketed paste', async (desk) => {
    if (desk.kind !== 'fake') return { pass: true, note: 'n/a: Claude folds a paste in its box; its exact lines are the long multi-line row' };
    const r = await typedBack(desk, LINES);
    return { pass: r.ok, note: `${r.res.status}; ${r.ok ? 'exact, \\e[200~ … \\e[201~ around it, no Return' : `got ${JSON.stringify(r.got)}`}` };
  }],
  ['multi-line where the pane takes no bracketed paste', async (desk) => {
    if (desk.kind !== 'fake') return { pass: true, note: 'n/a: Claude Code asks for bracketed paste at its prompt (not while a terminal $EDITOR has the tab, which the deck refuses); the stand-in can stop' };
    await dictate(desk.id, 'nopaste', true);
    await sleep(700);
    const mark = fakeMark(desk);
    const res = await dictate(desk.id, LINES, true);
    await sleep(700);
    const typed = fakeBytes(desk, mark);
    await dictate(desk.id, 'paste', true);
    await sleep(700);
    return { pass: res.status === 409 && res.body === NO_LINES && typed === '', note: `${res.status} ${res.body}; ${typed ? `TYPED ${JSON.stringify(typed)}` : 'nothing typed'}` };
  }],
  // I34: over the Wi-Fi's plain http, a paired phone's key types as through Tailscale; no key or a wrong one, nothing.
  ['plain LAN: typed with the paired key', async (desk) => {
    if (!LAN) return { pass: true, note: 'SKIPPED: set MATRIX_LAN' };
    const r = await typedBack(desk, FRENCH, { lan: true });
    return { pass: r.ok, note: `${r.res.status}; ${r.ok ? 'exact' : `got ${JSON.stringify(r.got)}`}` };
  }],
  ['plain LAN: no key, a wrong key, the peek', async (desk) => {
    if (!LAN) return { pass: true, note: 'SKIPPED: set MATRIX_LAN' };
    const before = desk.kind === 'fake' ? fakeMark(desk) : 0;
    const none = await dictate(desk.id, 'should not land', false, { lan: true, key: null });
    const wrong = await dictate(desk.id, 'should not land', false, { lan: true, key: 'not-a-paired-key' });
    const peek = await dictate(desk.id, '', false, { lan: true, path: '/peek' });
    await sleep(500);
    const landed = desk.kind === 'fake' ? fakeBytes(desk, before) !== '' : (boxLine(desk) ?? '').includes('should not land');
    return { pass: none.status === 401 && wrong.status === 401 && peek.status === 403 && !landed,
      note: `no key ${none.status}, wrong key ${wrong.status}, peek ${peek.status}; ${landed ? 'TEXT LANDED' : 'nothing typed'}` };
  }],
  // I5: the peek reads the input line through Tailscale only (the plain LAN row above); the same on both engines.
  ['the peek over Tailscale, after a line typed', async (desk) => {
    // A launcher's window sends the Go core its state, never its text: its peek is 409 "can't show" by design.
    if (desk.launcher) return { pass: true, note: "SKIPPED: the Go core can't peek a launcher's window (it says its state, never its text)" };
    const typed = await dictate(desk.id, SHORT, false);
    await sleep(700);
    const res = await dictate(desk.id, '', false, { path: '/peek' });
    let lines = [];
    try { lines = JSON.parse(res.body).lines ?? []; } catch {}
    const shown = lines.some((line) => line.includes(SHORT.slice(0, 20)));
    emptyBox(desk);
    await sleep(300);
    return { pass: typed.status === 204 && res.status === 200 && shown, note: `typed ${typed.status}; peek ${res.status}, ${lines.length} lines; ${shown ? 'the line is there' : 'NOT SHOWN'}` };
  }],
  // A typed answer to a menu goes as keys: one line only. No menu is open here, so one line is "gone", never typed.
  ['a typed answer: one line only, none with no menu', async (desk) => {
    const before = desk.kind === 'fake' ? fakeMark(desk) : 0;
    const lines = await dictate(desk.id, 'one\ntwo', false, { path: '/choose' });
    const none = await fetch(`${BASE}/choose?t=${encodeURIComponent(KEY)}&session=${encodeURIComponent(desk.id)}&option=other&choice=none`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: 'one line' }) });
    const several = await fetch(`${BASE}/choose?t=${encodeURIComponent(KEY)}&session=${encodeURIComponent(desk.id)}&option=other&choice=none`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: 'one\ntwo' }) });
    await sleep(500);
    const landed = desk.kind === 'fake' ? fakeBytes(desk, before) !== '' : /one line|two/.test(boxLine(desk) ?? '');
    return { pass: lines.status === 400 && none.status === 409 && several.status === 400 && !landed,
      note: `no option ${lines.status}, one line ${none.status}, several lines ${several.status}; ${landed ? 'TEXT LANDED' : 'nothing typed'}` };
  }],
  ['French accents + emoji (typed)', async (desk) => {
    const r = await typedBack(desk, FRENCH);
    return { pass: r.ok, note: `${r.res.status}; ${r.ok ? 'exact' : `got ${JSON.stringify(r.got)}`}` };
  }],
  ['quotes $HOME `…` $(…) ; && trailing \\ (typed)', async (desk) => {
    const r = await typedBack(desk, SHELLISH());
    const which = ['bt', 'cs'].filter(ran);
    return { pass: r.ok && !which.length, note: `${r.res.status}; ${r.ok ? 'exact' : `got ${JSON.stringify(r.got)}`}; ${which.length ? `RAN: ${which}` : 'nothing ran'}` };
  }],
  ['trailing ; (typed)', async (desk) => {
    const r = await typedBack(desk, SEMI);
    return { pass: r.ok, note: `${r.res.status}; ${r.ok ? 'exact' : `got ${JSON.stringify(r.got)}`}` };
  }],
  ...BANGS.map(([mark, name]) => [`leading !${mark ? ` + U+${mark.codePointAt(0).toString(16).toUpperCase().padStart(4, '0')}` : ''} (shell mode)`, async (desk) => {
    const res = await dictate(desk.id, `!${mark}touch ${canary(name)}`, false);
    await sleep(700);
    const box = boxLine(desk) ?? '';
    const shell = /for shell mode|bash mode/.test(screen(desk)) || box.includes('touch');
    emptyBox(desk); tmux(desk, 'send-keys', '-t', desk.target, 'BSpace');
    await sleep(300);
    return { pass: res.status === 400 && !shell && !ran(name), note: `${res.status} ${res.body}; ${shell ? 'IN THE BOX' : 'box empty'}` };
  }]),
  ['over 2000 characters', async (desk) => {
    const res = await dictate(desk.id, 'x'.repeat(2001), false);
    return { pass: res.status === 400 && /long/i.test(res.body), note: `${res.status} ${res.body}` };
  }],
  ['at a permission prompt', async (desk) => {
    // The agent asks to run something harmless; the phone types; the prompt must still be there, unanswered.
    if (desk.kind === 'fake') await dictate(desk.id, 'ask', true);
    else await dictate(desk.id, `I1 permission check: use the Bash tool to run exactly: touch ${canary('perm')}`, true);
    const state = await waitState(desk.id, ['asking'], 60000);
    if (state !== 'asking') return { pass: false, note: `never asked (${state})` };
    const before = screen(desk);
    const mark = desk.kind === 'fake' ? fakeMark(desk) : 0;
    const res = await dictate(desk.id, 'should not land', true);
    await sleep(800);
    const after = screen(desk);
    const landed = desk.kind === 'fake' ? fakeBytes(desk, mark) !== '' : after.includes('should not land');
    const still = await stateOf(desk.id) === 'asking' && (desk.kind === 'fake' ? after.includes('Allow the fake tool') : /Do you want to proceed|❯ 1\. Yes/.test(after));
    // Answered "no", so the desk is free again: Claude Code's from the phone's ✗ (the next case), the stand-in's by hand.
    if (desk.kind === 'fake') {
      tmux(desk, 'send-keys', '-t', desk.target, '2');
      await waitState(desk.id, ['idle', 'unread'], 30000);
    }
    return { pass: res.status === 409 && !landed && still && !ran('perm'),
      note: `${res.status} ${res.body}; prompt ${still ? 'still there' : 'GONE'}; ${landed ? 'TEXT LANDED' : 'nothing typed'}${before === after ? '; screen unchanged' : ''}${ran('perm') ? '; THE COMMAND RAN' : ''}` };
  }],
  ['✗ from the phone, then a follow-up typed', async (desk) => {
    if (desk.kind === 'fake') return { pass: true, note: 'n/a: ✗ is Claude Code\'s (the stand-in answers 1 or 2)' };
    const before = await stateOf(desk.id);
    const act = await fetch(`${BASE}/act?t=${encodeURIComponent(KEY)}&session=${encodeURIComponent(desk.id)}&action=reject`, { method: 'POST' });
    await sleep(800);
    const after = await stateOf(desk.id);
    const follow = 'I1 follow-up: reply with the single word ok.';
    const res = await dictate(desk.id, follow, true);
    await turn(desk.id);
    const got = lastPrompt(desk);
    return { pass: before === 'asking' && act.status === 204 && res.status === 204 && got === follow,
      note: `${before} → ✗ ${act.status} → ${after}; follow-up ${res.status} ${res.body}; prompt ${got === follow ? 'exact' : JSON.stringify(got)}` };
  }],
];

/** Last: the desk's agent gone, then its tmux. Each must be told, never a silent 204. */
const endings = [
  ['agent ended (SessionEnd), terminal still open', 'mgr', async (desk) => {
    tmux(desk, 'send-keys', '-t', desk.target, '/exit', 'Enter');
    await waitState(desk.id, ['gone'], 20000);
    const res = await dictate(desk.id, 'anyone there?', true);
    return { pass: res.status >= 400 && res.body.length > 0, note: `${res.status} ${res.body}` };
  }],
  ['agent killed (no SessionEnd)', 'fake', async (desk) => {
    const shell = tmux(desk, 'display-message', '-p', '-t', desk.target, '#{pane_pid}').stdout.trim();
    // The stand-in only: the one child of its own pane's shell, by its pid.
    const pid = /^\d+$/.test(shell) ? spawnSync('pgrep', ['-P', shell], { encoding: 'utf8' }).stdout.trim() : '';
    if (/^\d+$/.test(pid)) execFileSync('kill', ['-9', pid]);
    await sleep(1000);
    const res = await dictate(desk.id, 'anyone there?', true);
    return { pass: res.status >= 400 && res.body.length > 0, note: `${res.status} ${res.body}` };
  }],
  ['tmux server gone', 'hire', async (desk) => {
    tmux(desk, 'kill-server');
    await sleep(1000);
    const res = await dictate(desk.id, 'anyone there?', true);
    return { pass: res.status >= 400 && res.body.length > 0, note: `${res.status} ${res.body}` };
  }],
];

async function run(endingsOnly = false) {
  const only = process.env.MATRIX_ONLY?.split(',');
  const desks = JSON.parse(readFileSync(DESKS, 'utf8')).filter((d) => d.id && (!only || only.includes(d.name)));
  if (CANARIES.some(ran)) throw new Error('a canary from an earlier run is still there: remove it first');
  const rows = [];
  for (const desk of endingsOnly ? [] : desks) {
    for (const [name, check] of cases) {
      let result;
      try { result = await check(desk); } catch (e) { result = { pass: false, note: `error: ${e.message}` }; }
      rows.push([desk.name, name, result.pass ? 'PASS' : 'FAIL', result.note]);
      console.error(`${desk.name} · ${name}: ${result.pass ? 'PASS' : 'FAIL'} ${result.note}`);
    }
  }
  if (!process.env.MATRIX_KEEP) {
    for (const [name, deskName, check] of endings) {
      const desk = desks.find((d) => d.name === deskName);
      if (!desk) continue;
      let result;
      try { result = await check(desk); } catch (e) { result = { pass: false, note: `error: ${e.message}` }; }
      rows.push([desk.name, name, result.pass ? 'PASS' : 'FAIL', result.note]);
      console.error(`${desk.name} · ${name}: ${result.pass ? 'PASS' : 'FAIL'} ${result.note}`);
    }
  }
  for (const [name, what] of [['t3', 'T3 Code desk (T797b): text goes by `t3 thread send <id> -` on stdin; the same strings arrive exact in the thread; T3 not running → a plain refusal'],
    ['takeover', 'taken-over hire (T795): typing reaches the hire in its new home; the old window no longer takes it']]) {
    if (!desks.some((d) => d.name === name)) rows.push([name, what, 'PENDING', 'lane not merged']);
  }
  console.log('| desk | case | result | what came back |\n|---|---|---|---|');
  for (const r of rows) console.log(`| ${r.map((c) => String(c).replace(/\|/g, '\\|').replace(/\n/g, ' ')).join(' | ')} |`);
  process.exitCode = rows.some((r) => r[2] === 'FAIL') ? 1 : 0;
}

/**
 * Window mode, against background (no window): a client attached to the desk's own tmux session,
 * as a terminal window is, in a pseudo-terminal of its own (`script`), so no app is driven; the
 * same text typed; then that client detached, on the desk's socket only, and the session checked
 * still there. A real Terminal window attached the same way is the eyes-on step.
 */
async function windowMode(name) {
  const desk = JSON.parse(readFileSync(DESKS, 'utf8')).find((d) => d.name === name);
  // `script` wants a real pipe on its input; the pty is sized as the desk is, so the pane keeps its width.
  const attach = `stty cols 320 rows 45; exec tmux -L ${desk.socket} attach -t ${desk.target}`;
  const viewer = spawn('/bin/sh', ['-c', `sleep 120 | /usr/bin/script -q /dev/null /bin/sh -c '${attach}'`],
    { stdio: 'ignore', detached: true, env: { ...process.env, TERM: 'xterm-256color' } });
  await sleep(3000);
  const clients = tmux(desk, 'list-clients', '-t', desk.target).stdout.trim().split('\n').filter(Boolean).length;
  const r = await typedBack(desk, FRENCH);
  tmux(desk, 'detach-client', '-s', desk.target);
  await sleep(800);
  try { process.kill(-viewer.pid); } catch {} // its own group only: the sleep, script and the client
  const alive = tmux(desk, 'has-session', '-t', desk.target).status === 0;
  const left = tmux(desk, 'list-clients', '-t', desk.target).stdout.trim();
  console.log(`| ${name} | window mode (a client attached, ${clients} in all) | ${r.ok && clients === 1 && alive && !left ? 'PASS' : 'FAIL'} | ${r.res.status}; ${r.ok ? 'exact' : `got ${JSON.stringify(r.got)}`}; client detached; session ${alive ? 'still running' : 'GONE'} |`);
}

const command = process.argv[2];
if (command === 'up') await up();
else if (command === 'run') await run();
else if (command === 'end') await run(true);
else if (command === 'window') await windowMode(process.argv[3] || 'hire');
else if (command === 'down') down();
else console.error('matrix.mjs up | run | window <desk> | end | down');
