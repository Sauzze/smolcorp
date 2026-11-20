/*
 * The phone page's notifications worker (1.26 PUSH2), served by the computer at /sw.js with no key, the same file for
 * every phone. It holds no key and reads none: a paired phone's key stays in its page's address, after #, which a
 * worker never sees. It answers no request (it has no fetch handler), keeps nothing, and asks the computer nothing:
 * all it does is show what a push brings, a desk's event word, its name and its floor (push.go's pushNote), and
 * nothing else from it. What a tap on a notification opens is PUSH4's.
 */

/** What each event says after the desk's name: the three a phone can be rung for (push.go's pushEvents). */
const SAYS = { waiting: 'needs you', done: 'is done', error: 'stopped with an error' };
/** The longest desk name shown, in characters, as push.go's pushDeskMost. */
const DESK_MOST = 64;

/**
 * A notification's title and options for what a push brought: only its event, desk and floor are read, each checked,
 * and anything else in it is left out. A push that isn't one of ours still shows a notification (a browser takes
 * its leave away from a worker that shows none), with words of its own.
 */
function noteOf(data) {
  const said = data && typeof data === 'object' ? data : {};
  const event = Object.prototype.hasOwnProperty.call(SAYS, said.event) ? said.event : '';
  const desk = typeof said.desk === 'string' ? Array.from(said.desk.trim()).slice(0, DESK_MOST).join('') : '';
  const floor = Number.isInteger(said.floor) && said.floor > 0 && said.floor < 6 ? said.floor : 0;
  if (!event || !desk) return { title: 'Smolcorp', options: { body: 'Something in the office needs a look.', icon: '/icon.png' } };
  // Floor 0 is the office itself, the ground floor; the office counts that one as the first.
  const where = floor ? ` · floor ${floor + 1}` : '';
  return { title: 'Smolcorp', options: { body: `${desk} ${SAYS[event]}${where}`, icon: '/icon.png' } };
}

self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()));
self.addEventListener('push', (event) => {
  let data = null;
  try {
    data = event.data ? event.data.json() : null;
  } catch {}
  const { title, options } = noteOf(data);
  event.waitUntil(self.registration.showNotification(title, options));
});
