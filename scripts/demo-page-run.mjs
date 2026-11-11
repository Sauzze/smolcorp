// The browser's side of scripts/demo-page-run.sh (FXX1d): headless Chrome on 127.0.0.1, no window. It signs in
// with the test copy's own one-time link (as the menu's Settings opens Setup), opens the link `smolcorp demo
// --no-open` printed, checks the demo's card, clicks Hire, and checks what follows through the routes:
//
//   node scripts/demo-page-run.mjs RUN PORT CDP LINK CLI DEMO_PID      (demo-page-run.sh runs it)
//
// RUN is build/dp, PORT the copy's, CDP Chrome's debugging port, LINK the demo's page, CLI the copy's smolcorp
// command, DEMO_PID the demo's process (Ctrl-C'd at the end). Prints PASS/FAIL lines; exits 1 on any FAIL.
import { spawn, spawnSync } from 'node:child_process';
import { readFileSync, existsSync, mkdirSync, appendFileSync } from 'node:fs';
import { join } from 'node:path';

const [RUN, PORT, CDP, LINK, CLI, DEMO_PID] = process.argv.slice(2);
const CHROME = process.env.DEMO_CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const home = join(RUN, 'home');
const root = new URL(LINK).searchParams.get('demo');
const repo = join(root, 'app');
let failed = 0;
const check = (pass, what, note = '') => {
  if (!pass) failed++;
  const line = `${pass ? 'PASS' : 'FAIL'} ${what}${note ? `: ${note}` : ''}`;
  console.log(line);
  appendFileSync(join(RUN, 'run.log'), line + '\n');
};
const demoOut = () => readFileSync(join(RUN, 'demo.out'), 'utf8');
async function until(what, test, ms = 30000) {
  for (const end = Date.now() + ms; Date.now() < end; await sleep(250)) {
    const value = await test();
    if (value) return value;
  }
  return null;
}
// A tool's call, with the setup key from the copy's own folder (as the demo's command makes it).
// A JSON answer, or {} for one that isn't (the team is closed while the manager is off: 409 in words).
const json = (text) => { try { return JSON.parse(text); } catch { return {}; } };
async function asTool(path) {
  const key = readFileSync(join(RUN, 'support', 'setup-key'), 'utf8').trim();
  const res = await fetch(`http://127.0.0.1:${PORT}${path}`, { headers: { 'X-Smolcorp-Setup-Key': key, 'X-AgentDeck-Setup': '1' } });
  return { status: res.status, body: await res.text() };
}

/* ------------------------------------------------------------- Chrome, headless, on 127.0.0.1 only */

mkdirSync(join(RUN, 'chrome'), { recursive: true });
// Never the Keychain, nor any other system dialog (_rules.md, owner 11 Nov 15:01): a mock keychain, passwords in its
// own profile, no casting; a profile and a home of its own inside build/dp.
mkdirSync(join(RUN, 'chrome-home'), { recursive: true });
const browser = spawn(CHROME, ['--headless=new', '--use-mock-keychain', '--password-store=basic', '--disable-features=DialMediaRouteProvider',
  `--remote-debugging-port=${CDP}`, '--remote-debugging-address=127.0.0.1', `--user-data-dir=${join(RUN, 'chrome')}`,
  '--no-first-run', '--no-default-browser-check', 'about:blank'],
  { stdio: 'ignore', env: { ...process.env, HOME: join(RUN, 'chrome-home'), CFFIXED_USER_HOME: join(RUN, 'chrome-home') } });
let session;
try {
  let target;
  for (let i = 0; i < 75 && !target; i++) {
    await sleep(200);
    target = await fetch(`http://127.0.0.1:${CDP}/json`).then((r) => r.json()).then((l) => l.find((t) => t.type === 'page')).catch(() => null);
  }
  if (!target) throw new Error("Chrome's debugging port didn't answer");
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener('open', r, { once: true }));
  let next = 0;
  const waiting = new Map();
  const errors = [];
  ws.addEventListener('message', ({ data }) => {
    const m = JSON.parse(data);
    if (m.id && waiting.has(m.id)) { waiting.get(m.id)(m); waiting.delete(m.id); }
    if (m.method === 'Runtime.exceptionThrown') errors.push(m.params.exceptionDetails.exception?.description ?? m.params.exceptionDetails.text);
  });
  const send = (method, params = {}) => new Promise((r) => { const id = ++next; waiting.set(id, r); ws.send(JSON.stringify({ id, method, params })); });
  const run = async (expression) => (await send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })).result?.result?.value;
  const goTo = async (url) => { await send('Page.navigate', { url }); await sleep(1500); };
  session = { ws };
  await send('Runtime.enable');
  await send('Page.enable');

  // 1. Signed in as the menu's Settings does: the copy's own one-time link (a test copy writes it as it opens).
  await goTo(readFileSync(join(RUN, 'support', 'setup-link'), 'utf8').trim());
  check(await until('signed in', () => run(`!!document.querySelector('#start-here') && !document.body.dataset.locked`), 10000), 'Setup opens signed in with the copy\'s one-time link');

  // 2. The demo's link: the card, filled in by the app from the demo folder.
  await goTo(LINK);
  const card = await until('the card', () => run(`(() => { const c = document.getElementById('demo-card'), h = document.getElementById('demo-hire');
    return c && !c.hidden && h && !h.hidden && { folder: document.getElementById('demo-folder').textContent, scope: document.getElementById('demo-scope').textContent,
      roles: document.getElementById('demo-roles').textContent, switches: document.getElementById('demo-switches').textContent,
      task: document.getElementById('demo-task').textContent, address: location.pathname + location.search }; })()`), 15000);
  check(!!card && card.folder === repo && card.scope === repo, "the demo's card shows the project and its scope", JSON.stringify(card && { folder: card.folder, scope: card.scope }));
  check(!!card && card.switches.includes("Hire turns on Smolcorp's manager and hiring"), 'it names the switches Hire turns on', card && card.switches);
  check(!!card && card.task.includes(`--folder ${repo}`) && card.roles.startsWith('reviewer'), 'its task and roles are the demo\'s', card && card.roles);
  check(!!card && card.address === new URL(LINK).pathname + new URL(LINK).search, 'the address keeps ?demo=', card && card.address);
  check(await until('page open', async () => json((await asTool(`/setup/demo?root=${encodeURIComponent(root)}`)).body).demo?.page === 'open', 5000), 'the page told the app it is open');
  check(!demoOut().includes('hired the General Manager'), 'nothing is hired before the click');

  const before = json((await asTool('/setup/state')).body);
  check(before.choices?.background === true, "the copy runs hires in the background (no terminal window)", JSON.stringify(before.choices));
  // 3. Hire, clicked.
  await run(`document.getElementById('demo-hire').click(); true`);
  const said = await until('hired', () => run(`(() => { const s = document.getElementById('demo-said').textContent; return /^Hired|^Not hired/.test(s) && s; })()`), 30000);
  check(!!said && said.startsWith('Hired'), 'Hire hires the demo\'s General Manager', said);

  // 4. The GM, held to its project, as the page's hire asked; both switches on.
  // The app's own registry, in the copy's folder (GET /setup/gm/registry isn't served on Go).
  const registry = await until('the GM\'s row', () => {
    const file = json(readFileSync(join(RUN, 'support', 'registry.json'), 'utf8'));
    return (file.hires || []).some((h) => h.role === 'gm') && file;
  }, 10000);
  const gm = (registry?.hires || []).find((h) => h.role === 'gm' && h.state !== 'fired' && h.state !== 'failed');
  check(!!gm && gm.folder === repo && gm.scope === repo, 'the GM is hired into the project, with its scope', JSON.stringify(gm && { folder: gm.folder, scope: gm.scope, state: gm.state, account: gm.account }));
  const state = json((await asTool('/setup/state')).body);
  check(state.manager?.enabled && state.hiring?.enabled, "the page's Hire turned the manager and hiring on");

  // 5. The demo's terminal follows: it saw the hire, then the GM in the team.
  check(!!(await until('following', () => demoOut().includes('You hired the General Manager') && /\(gm\): (starting|working)/.test(demoOut()), 30000)),
    "the demo's terminal saw the hire and follows the GM", (demoOut().match(/.*\(gm\).*/g) || []).join(' | '));
  const working = await until('working', async () => json((await asTool('/setup/gm/team')).body).team?.find((m) => m.role === 'gm' && m.state === 'working'), 60000);
  check(!!working, 'the stand-in GM reported in and is working', working && working.state);

  // 6. A scope refusal through the routes, with the GM's own token (its start script gave it to the stand-in).
  const token = await until('the GM\'s token', () => existsSync(join(RUN, 'gm-token')) && readFileSync(join(RUN, 'gm-token'), 'utf8').trim(), 10000);
  if (token) {
    const outside = join(home, 'outside');
    mkdirSync(outside, { recursive: true });
    const asGM = (...args) => spawnSync(CLI, ['gm', ...args], { encoding: 'utf8', env: { ...process.env, SMOLCORP_GM_TOKEN: token, SMOLCORP_PORT: PORT } });
    const refused = asGM('hire', '--role', 'reviewer', '--folder', outside);
    const said = `${refused.stdout}${refused.stderr}`.trim();
    check(refused.status !== 0 && said.includes('This General Manager'), "the GM's hire outside its project is refused by the app", said);
    const switched = await fetch(`http://127.0.0.1:${PORT}/setup/hiring?enabled=0`, { method: 'POST', headers: { 'X-Smolcorp-GM-Token': token, 'X-AgentDeck-Setup': '1' } });
    const why = await switched.text();
    check(switched.status === 403 && why.startsWith('This General Manager'), 'the GM can\'t turn a switch off either', `${switched.status} ${why}`);
  } else check(false, "the GM's token reached the stand-in");
  // The demo's command, a tool, still can't hire.
  const toolHire = await fetch(`http://127.0.0.1:${PORT}/setup/hire`, { method: 'POST', body: '{}',
    headers: { 'X-Smolcorp-Setup-Key': readFileSync(join(RUN, 'support', 'setup-key'), 'utf8').trim(), 'X-AgentDeck-Setup': '1', 'Content-Type': 'application/json' } });
  check(toolHire.status === 403, 'the setup key still can\'t hire (the route table is unchanged)', String(toolHire.status));

  // 7. Fired from the page, and the demo's terminal stopped with Ctrl-C.
  if (gm) {
    const fired = await run(`fetch('/setup/gm/fire', { method: 'POST', headers: { 'X-AgentDeck-Setup': '1', 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: ${JSON.stringify(gm.id)}, force: true }) }).then(async (r) => r.status + ' ' + (await r.text()).slice(0, 200))`);
    check(/^2\d\d/.test(fired), 'the GM is fired from the page', fired);
    check(!!(await until('fired', async () => !(json((await asTool('/setup/gm/team')).body).team || []).some((m) => m.role === 'gm' && m.state !== 'fired' && m.state !== 'failed'), 20000)),
      'the team has no GM left');
  }
  process.kill(Number(DEMO_PID), 'SIGINT');
  check(!!(await until('stopped', () => demoOut().includes('Stopped following'), 10000)), "Ctrl-C stops the demo's terminal following");
  check(errors.length === 0, 'no script error on the page', errors.join(' | ').slice(0, 300));
} catch (e) {
  check(false, 'the run', String(e.stack || e));
} finally {
  session?.ws.close();
  browser.kill();
  await sleep(500);
}
process.exit(failed ? 1 : 0);
