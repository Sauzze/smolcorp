// A stand-in for a non-Claude coding agent at a desk, for scripts/input-matrix/matrix.mjs.
// Run as `exec -a codex node fake-agent.mjs <log> <hook>` inside a tmux pane, so the
// app sees a Codex in front there. <hook> is a command line, run by /bin/sh with "codex" after
// it and the event on its stdin: the Mac test copy's quoted report.sh, or the Go core's
// `smolcorp hook --port N`. It reports in through that hook, as Codex's
// hooks would, and writes every byte it receives to <log>, one JSON line per read:
//   {"at":…, "bytes":"<hex>", "text":"…", "state":"idle"|"asking"}
// and one line per submitted prompt: {"submit":"…"}. Nothing it reads is ever run.
//
// Its input box takes bracketed pastes and keys like a real agent's. A prompt of exactly
// `ask` makes it ask a permission question (state "asking"), which only 1 or 2 answers;
// `/exit` ends it with SessionEnd, as an agent that quits.
import { appendFileSync } from 'node:fs';
import { spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { StringDecoder } from 'node:string_decoder';

const [log, report] = process.argv.slice(2);
const session = randomUUID();
let state = 'idle';
let box = '';
let pasting = false;
// A character may come in two reads.
const decoder = new StringDecoder('utf8');

function hook(event, extra = {}) {
  const body = JSON.stringify({ hook_event_name: event, session_id: session, cwd: process.cwd(), ...extra });
  const child = spawn('/bin/sh', ['-c', `${report} codex`], { stdio: ['pipe', 'ignore', 'ignore'] });
  child.stdin.end(body);
}

function draw() {
  const top = state === 'asking' ? 'Allow the fake tool to run?\r\n  1. Yes\r\n  2. No\r\n' : '';
  process.stdout.write(`\x1b[2J\x1b[H${top}fake-codex> ${box}`);
}

function submit() {
  const prompt = box;
  box = '';
  appendFileSync(log, JSON.stringify({ submit: prompt }) + '\n');
  if (prompt === '/exit') {
    hook('SessionEnd', { reason: 'prompt_input_exit' });
    setTimeout(() => process.exit(0), 300);
    return;
  }
  hook('UserPromptSubmit', { prompt });
  if (prompt === 'ask') {
    state = 'asking';
    // After the prompt's own event: hooks run side by side, and arrive in any order.
    setTimeout(() => hook('PermissionRequest', { tool_name: 'Bash', tool_input: { command: 'true' } }), 300);
  } else {
    setTimeout(() => hook('Stop'), 200);
  }
  draw();
}

process.stdin.setRawMode(true);
process.stdout.write('\x1b[?2004h'); // asks for bracketed paste, as real agents do
hook('SessionStart', { source: 'startup' });
draw();

process.stdin.on('data', (chunk) => {
  appendFileSync(log, JSON.stringify({ at: Date.now(), bytes: chunk.toString('hex'), text: chunk.toString('utf8'), state }) + '\n');
  let text = decoder.write(chunk);
  while (text.length) {
    if (text.startsWith('\x1b[200~')) { pasting = true; text = text.slice(6); continue; }
    if (text.startsWith('\x1b[201~')) { pasting = false; text = text.slice(6); continue; }
    const ch = text[0];
    text = text.slice(1);
    if (state === 'asking') {
      // Only an option answers; anything else is ignored, as a real menu does.
      if (ch === '1' || ch === '2') {
        state = 'idle';
        hook('PostToolUse', { tool_name: 'Bash' });
        setTimeout(() => hook('Stop'), 200);
      }
      continue;
    }
    if (ch === '\r' && !pasting) { submit(); continue; }
    if (ch === '\x03') { hook('SessionEnd', { reason: 'other' }); process.exit(0); }
    if (ch === '\x15') { box = ''; continue; } // Ctrl+U empties the box
    if (ch === '\x7f') { box = [...box].slice(0, -1).join(''); continue; }
    box += ch;
  }
  draw();
});
