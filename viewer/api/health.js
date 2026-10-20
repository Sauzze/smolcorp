'use strict';
/*
 * /api/health
 *   GET  Bearer   checks the settings and Redis, for the setup page's "Check the viewer"
 *   204 ready; 401; 429; 503 with a short reason, never a value.
 *
 * A wrong bearer counts in the same bucket as refused writes, so this can't
 * be used to guess the write secret faster than a PUT could. Until the bearer
 * is good, every 503 is the same plain "not configured": what's missing is
 * only said to the Mac.
 */
const lib = require('./_lib.js');

module.exports = async function handler(req, res) {
  if (req.method !== 'GET') return lib.notAllowed(res, 'GET');
  const auth = lib.authorize(req);
  if (auth === 'unset' || (auth !== 'ok' && !lib.redisConfigured())) return lib.reply(res, 503, 'not configured');
  if (auth !== 'ok') {
    try {
      if ((await lib.counted(req, 'bad')).over) return lib.reply(res, 429, 'too many requests');
    } catch {
      return lib.reply(res, 503, 'unavailable');
    }
    return lib.reply(res, 401, 'unauthorized');
  }
  if (!lib.redisConfigured()) return lib.reply(res, 503, 'Redis is not connected (KV_REST_API_URL / KV_REST_API_TOKEN)');
  try {
    const [pong] = await lib.redis([['PING']]);
    if (pong !== 'PONG') return lib.reply(res, 503, 'Redis did not answer');
  } catch {
    return lib.reply(res, 503, 'Redis did not answer');
  }
  return lib.reply(res, 204);
};
