// The phone's side of scripts/input-matrix: the real deck page in headless Chrome, as an
// iPhone (390×844, touch, its user agent), over the Wi-Fi's plain http and over HTTPS as
// through `tailscale serve` (tls-proxy.mjs). The page's own code does the work: a desk picked
// as a tap picks it, the mic held and let go, what it heard shown in the confirm box, typed
// over, then Type only or Send ⏎. Only the speech recognition is a stand-in (it says what the
// test gives it): the browser's own sends audio to its maker's servers, so a real round trip
// from the mic is checked by hand on a phone.
//
//   node scripts/input-matrix/page.mjs     after `matrix.mjs up`, with the same MATRIX_* settings
//   MATRIX_CHROME   Chrome's binary (/Applications/Google Chrome.app/Contents/MacOS/Google Chrome)
//   MATRIX_CDP      its debugging port (7781)
//   MATRIX_LAN      the copy's address on the Wi-Fi (http://<this computer's LAN IP>:<port>)
import { spawn, spawnSync, execFileSync } from 'node:child_process';
import { readFileSync, existsSync, mkdirSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, '../..');
const PORT = process.env.MATRIX_PORT || '7780';
const DIR = process.env.MATRIX_DIR || join(repo, 'build/input-matrix');
const KEY = readFileSync(process.env.MATRIX_KEY, 'utf8').trim();
const CHROME = process.env.MATRIX_CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const CDP = process.env.MATRIX_CDP || '7781';
const LAN = process.env.MATRIX_LAN || `http://${execFileSync('ipconfig', ['getifaddr', 'en0'], { encoding: 'utf8' }).trim()}:${PORT}`;
const PREFIX = process.env.MATRIX_SOCKETS || 'i1';
const IPHONE = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const desks = JSON.parse(readFileSync(join(DIR, 'desks.json'), 'utf8'));
const fake = desks.find((d) => d.name === 'fake');
const claude = desks.find((d) => d.name === 'gm');

/* ------------------------------------------------------------- before the page's own scripts */

// The browser's speech recognition, saying `window.__say`; and the office's callbacks, as its taps call them.
// In a scope of its own: the page's scripts declare names of their own at the top.
const PRELOAD = `(() => {
  window.__say = '';
  class FakeRecognition {
    start() { setTimeout(() => this.onresult && this.onresult({ results: [Object.assign([{ transcript: window.__say }], { isFinal: true })] }), 150); }
    stop() { setTimeout(() => this.onend && this.onend(), 50); }
    abort() {}
  }
  window.SpeechRecognition = window.webkitSpeechRecognition = FakeRecognition;
  let office;
  Object.defineProperty(window, 'AgentOffice', { configurable: true, get: () => office, set(value) {
    const create = value.create;
    value.create = (canvas, on) => { window.__on = on; return create.call(value, canvas, on); };
    office = value;
  } });
})();`;

/* ------------------------------------------------------------- Chrome, through its debugging port */

async function chrome() {
  mkdirSync(join(DIR, 'chrome'), { recursive: true });
  const browser = spawn(CHROME, ['--headless=new', `--remote-debugging-port=${CDP}`, `--user-data-dir=${join(DIR, 'chrome')}`,
    '--ignore-certificate-errors', '--no-first-run', '--no-default-browser-check', '--use-mock-keychain', '--password-store=basic', '--disable-features=DialMediaRouteProvider', 'about:blank'], { stdio: 'ignore' });
  let target;
  for (let i = 0; i < 50 && !target; i++) {
    await sleep(200);
    target = await fetch(`http://127.0.0.1:${CDP}/json`).then((r) => r.json()).then((l) => l.find((t) => t.type === 'page')).catch(() => null);
  }
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener('open', r, { once: true }));
  let next = 0;
  const waiting = new Map();
  const requests = [];
  ws.addEventListener('message', ({ data }) => {
    const m = JSON.parse(data);
    if (m.id && waiting.has(m.id)) { waiting.get(m.id)(m); waiting.delete(m.id); }
    if (m.method === 'Runtime.exceptionThrown') console.error(`page error: ${m.params.exceptionDetails.exception?.description ?? m.params.exceptionDetails.text}`.slice(0, 300));
    if (m.method === 'Network.requestWillBeSent') requests.push({ id: m.params.requestId, url: m.params.request.url, body: m.params.request.postData });
    if (m.method === 'Network.responseReceived') {
      const r = requests.find((x) => x.id === m.params.requestId);
      if (r) r.status = m.params.response.status;
    }
  });
  const send = (method, params = {}) => new Promise((r) => { const id = ++next; waiting.set(id, r); ws.send(JSON.stringify({ id, method, params })); });
  const run = async (expression) => (await send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })).result?.result?.value;
  await send('Network.enable');
  await send('Runtime.enable');
  await send('Page.enable');
  await send('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 3, mobile: true });
  await send('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 5 });
  await send('Network.setUserAgentOverride', { userAgent: IPHONE, platform: 'iPhone' });
  await send('Page.addScriptToEvaluateOnNewDocument', { source: PRELOAD });
  return { browser, send, run, requests, close: () => { ws.close(); browser.kill(); } };
}

/* ------------------------------------------------------------- the stand-in's log */

const logRows = () => readFileSync(fake.log, 'utf8').split('\n').filter(Boolean).map((l) => JSON.parse(l));
const pastedSince = (from) => Buffer.concat(logRows().slice(from).filter((r) => r.bytes).map((r) => Buffer.from(r.bytes, 'hex'))).toString('utf8');
const tmux = (desk, ...args) => {
  if (!desk.socket.startsWith(`${PREFIX}-`)) throw new Error(`not our socket: ${desk.socket}`);
  return spawnSync('tmux', ['-L', desk.socket, ...args], { encoding: 'utf8' });
};

/** The page's mic held on `desk` and let go, hearing `words`; the confirm box's state after. */
async function hold(page, desk, words) {
  await page.run(`window.__say = ${JSON.stringify(words)}; window.__on.select(${JSON.stringify(desk.id)}); true`);
  await sleep(400);
  await page.run(`window.__on.mic('down'); true`);
  await sleep(600);
  await page.run(`window.__on.mic('up'); true`);
  await sleep(500);
  return page.run(`({ open: !document.getElementById('confirm').hidden, text: document.getElementById('confirm-text').value,
    sendHeld: document.getElementById('confirm-send').disabled, why: !document.getElementById('confirm-why').hidden })`);
}
/** Typed over what the box holds, as the phone's keyboard types, then one of the box's buttons tapped. */
async function typeAndTap(page, text, button) {
  if (text !== null) {
    // Emptied, then typed into, as the keyboard types: insertText goes where the caret is.
    await page.run(`{ const box = document.getElementById('confirm-text'); box.focus(); box.value = ''; } true`);
    await page.send('Input.insertText', { text });
  }
  const before = dictations(page).length;
  await page.run(`document.getElementById(${JSON.stringify(button)}).click(); true`);
  // Until the computer answers, however long its terminal queue takes; a tap that sends nothing, 2 s.
  for (let i = 0; i < 400; i++) {
    await sleep(500);
    const sent = dictations(page).slice(before);
    if (sent.length ? sent.every((r) => r.status) : i >= 4) break;
  }
  await sleep(300);
  return page.run(`document.getElementById('status').textContent`);
}
const dictations = (page) => page.requests.filter((r) => r.url.includes('/dictate'));
/** Which desk the page's last /dictate was for, by its session id, and its answer. */
const lastSent = (page) => { const r = dictations(page).at(-1); return r ? `to ${new URL(r.url).searchParams.get('session')?.slice(0, 8)} ${r.status}` : 'nothing sent'; };

/* ------------------------------------------------------------- the cases */

const rows = [];
const row = (where, name, pass, note) => { rows.push([where, name, pass ? 'PASS' : 'FAIL', note]); console.error(`${where} · ${name}: ${pass ? 'PASS' : 'FAIL'} ${note}`); };

const page = await chrome();
const cert = join(DIR, 'cert.pem'), key = join(DIR, 'key.pem');
if (!existsSync(cert)) {
  execFileSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', key, '-out', cert, '-days', '1', '-subj', '/CN=localhost'], { stdio: 'ignore' });
}
const proxy = spawn(process.execPath, [join(here, 'tls-proxy.mjs'), PORT, cert, key], { stdio: ['ignore', 'pipe', 'ignore'] });
const proxyPort = await new Promise((r) => proxy.stdout.once('data', (d) => r(String(d).trim())));
try {
  // Over the Wi-Fi's http: no phone mic, no typing; the mic is the computer's.
  await page.send('Page.navigate', { url: `${LAN}/?t=${encodeURIComponent(KEY)}` });
  await sleep(4000);
  const lan = await page.run(`({ office: !!window.__on, secure: window.isSecureContext, protocol: location.protocol })`);
  const lanBox = await hold(page, fake, 'over the wifi');
  const lanAct = page.requests.filter((r) => r.url.includes('action=mic')).map((r) => r.status);
  row('LAN http', 'the mic is the computer\'s, no box, no /dictate', lan?.office && !lanBox.open && dictations(page).length === 0 && lanAct.length === 2,
    `office ${lan?.office}, secure ${lan?.secure}; box ${lanBox.open ? 'OPEN' : 'not shown'}; /act mic ${lanAct.join(', ')}; /dictate ${dictations(page).length}`);
  const direct = await page.run(`fetch('/dictate?t=${encodeURIComponent(KEY)}&session=${fake.id}', { method: 'POST', body: JSON.stringify({ text: 'x' }) })
    .then(async (r) => r.status + ' ' + await r.text())`);
  row('LAN http', '/dictate straight from the Wi-Fi', direct?.startsWith('403'), direct);

  // Over HTTPS, as through tailscale serve.
  page.requests.length = 0;
  await page.send('Page.navigate', { url: `https://127.0.0.1:${proxyPort}/?t=${encodeURIComponent(KEY)}` });
  await sleep(4000);
  const tls = await page.run(`({ office: !!window.__on, secure: window.isSecureContext })`);
  row('HTTPS', 'the page loads as the phone\'s', tls?.office && tls?.secure, JSON.stringify(tls));

  const heard = 'Bonjour l’équipe, ça avance ?';
  let box = await hold(page, fake, heard);
  row('HTTPS · fake', 'what was heard shows in the box', box.open && box.text === heard, JSON.stringify(box));
  const typed = `Fin de phrase « é è à ç ô œ » 🎉 👩‍💻 "q" 'q' $HOME \`touch i1-canary-page\` $(touch i1-canary-page) ; a && b \\ ;`;
  let mark = logRows().length;
  let status = await typeAndTap(page, typed, 'confirm-type');
  let got = pastedSince(mark);
  row('HTTPS · fake', 'typed over, Type only', got === `\x1b[200~${typed}\x1b[201~` && !existsSync(join(repo, 'build/input-matrix-desks/fake/i1-canary-page')),
    `${lastSent(page)}; status "${status}"; ${got === `\x1b[200~${typed}\x1b[201~` ? 'exact' : `got ${JSON.stringify(got)}`}`);
  tmux(fake, 'send-keys', '-t', fake.target, 'C-u');

  box = await hold(page, fake, 'envoie ce message court');
  mark = logRows().length;
  status = await typeAndTap(page, null, 'confirm-send');
  await sleep(800);
  const submitted = logRows().slice(mark).find((r) => 'submit' in r)?.submit;
  const peek = await page.run(`({ shown: !document.getElementById('peek').hidden, text: document.getElementById('peek-text').textContent })`);
  row('HTTPS · fake', 'heard, Send ⏎', submitted === 'envoie ce message court', `status "${status}"; submitted ${JSON.stringify(submitted)}; peek ${peek.shown ? 'shown' : 'not shown'}`);

  // Asking: the box holds both buttons, and a tap sends nothing.
  await fetch(`http://127.0.0.1:${PORT}/dictate?t=${encodeURIComponent(KEY)}&session=${fake.id}`, { method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Forwarded-For': '100.101.102.103' }, body: JSON.stringify({ text: 'ask', send: true }) });
  await sleep(1500);
  const before = dictations(page).length;
  mark = logRows().length;
  box = await hold(page, fake, 'pendant la question');
  status = await typeAndTap(page, null, 'confirm-send');
  const asking = tmux(fake, 'capture-pane', '-p', '-t', fake.target).stdout.includes('Allow the fake tool');
  row('HTTPS · fake', 'at a permission prompt', box.sendHeld && box.why && dictations(page).length === before && pastedSince(mark) === '' && asking,
    `Send ${box.sendHeld ? 'held' : 'NOT HELD'}, why ${box.why ? 'shown' : 'hidden'}; ${dictations(page).length - before} sent; prompt ${asking ? 'still there' : 'GONE'}`);
  await page.run(`document.getElementById('confirm-cancel').click(); true`);
  tmux(fake, 'send-keys', '-t', fake.target, '2');
  await sleep(1000);

  // A Claude Code desk, typed into from the box.
  if (claude?.id) {
    box = await hold(page, claude, 'pour le GM');
    status = await typeAndTap(page, 'Pour le GM : « ça marche » 👩‍💻 ;', 'confirm-type');
    await sleep(600);
    const line = tmux(claude, 'capture-pane', '-p', '-t', claude.target).stdout.split('\n').findLast((l) => /^❯[  ]/.test(l)) ?? '';
    const want = 'Pour le GM : « ça marche » 👩‍💻 ;';
    row('HTTPS · gm', 'typed over, Type only', line.replace(/^❯[  ]/, '').trimEnd() === want, `status "${status}"; box ${JSON.stringify(line.trimEnd())}`);
    tmux(claude, 'send-keys', '-t', claude.target, 'C-e', 'C-u');
  }

  // The agent gone: the page says so.
  const shell = tmux(fake, 'display-message', '-p', '-t', fake.target, '#{pane_pid}').stdout.trim();
  const agent = /^\d+$/.test(shell) ? spawnSync('pgrep', ['-P', shell], { encoding: 'utf8' }).stdout.trim() : '';
  if (/^\d+$/.test(agent)) execFileSync('kill', ['-9', agent]);
  await sleep(1000);
  box = await hold(page, fake, 'tu es là ?');
  status = await typeAndTap(page, null, 'confirm-send');
  row('HTTPS · fake', 'its agent gone (killed)', /ended|not found|gone/i.test(status), `status "${status}"`);
} finally {
  page.close();
  proxy.kill();
}
console.log('| where | case | result | what came back |\n|---|---|---|---|');
for (const r of rows) console.log(`| ${r.map((c) => String(c).replace(/\|/g, '\\|')).join(' | ')} |`);
process.exitCode = rows.some((r) => r[2] === 'FAIL') ? 1 : 0;
