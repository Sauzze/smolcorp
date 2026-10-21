'use strict';
/*
 * /api/s
 *   DELETE  Bearer   deletes every s:* key (SCAN + DEL): "wipe the viewer"   204 / 401 / 429 / 503
 */
const lib = require('../_lib.js');

module.exports = async function handler(req, res) {
  if (req.method !== 'DELETE') return lib.notAllowed(res, 'DELETE');
  try {
    const auth = lib.authorize(req);
    if (auth === 'unset' || !lib.redisConfigured()) return lib.reply(res, 503, 'not configured');
    // The bearer first: a refused wipe counts in its own bucket, never the Mac's.
    const kind = auth === 'ok' ? 'write' : 'bad';
    if ((await lib.counted(req, kind)).over) return lib.reply(res, 429, 'too many requests');
    if (auth !== 'ok') return lib.reply(res, 401, 'unauthorized');
    let cursor = '0';
    let rounds = 0;
    do {
      const [[next, keys]] = await lib.redis([['SCAN', cursor, 'MATCH', 's:*', 'COUNT', 500]]);
      cursor = String(next);
      if (Array.isArray(keys) && keys.length) await lib.redis([['DEL', ...keys]]);
      rounds += 1;
    } while (cursor !== '0' && rounds < 1000);
    return lib.reply(res, 204);
  } catch {
    return lib.reply(res, 503, 'unavailable');
  }
};
