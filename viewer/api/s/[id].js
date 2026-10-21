'use strict';
/*
 * /api/s/:id
 *   PUT     Bearer   stores the body for 10 minutes        204 / 400 / 401 / 413 / 429 / 503
 *   GET     none     returns it                            200 / 404 / 429 / 503
 *   DELETE  Bearer   deletes it                            204 / 401 / 429 / 503
 */
const lib = require('../_lib.js');

module.exports = async function handler(req, res) {
  try {
    if (req.method === 'GET') return await read(req, res);
    if (req.method === 'PUT') return await write(req, res);
    if (req.method === 'DELETE') return await remove(req, res);
    return lib.notAllowed(res, 'GET, PUT, DELETE');
  } catch {
    return lib.reply(res, 503, 'unavailable');
  }
};

async function read(req, res) {
  if (!lib.redisConfigured()) return lib.reply(res, 503, 'unavailable');
  const id = lib.idFrom(req);
  // A malformed id is never one of ours: the same 404, without asking Redis.
  // Ids are 128 random bits, so how fast it comes says nothing worth knowing.
  if (!lib.ID.test(id)) return lib.reply(res, 404, 'not found');
  const { over, results: [stored] } = await lib.counted(req, 'read', [['GET', `s:${id}`]]);
  if (over) return lib.reply(res, 429, 'too many requests');
  if (typeof stored !== 'string' || !stored) return lib.reply(res, 404, 'not found');
  return lib.reply(res, 200, Buffer.from(stored, 'base64'), 'application/octet-stream');
}

/** The checks every write shares: 503 with no secret or no Redis, 429, 401. True when it may go on. */
async function writable(req, res) {
  const auth = lib.authorize(req);
  if (auth === 'unset' || !lib.redisConfigured()) { lib.reply(res, 503, 'not configured'); return false; }
  // The bearer first (local, constant time): a refused write counts in its
  // own bucket and never against the Mac's.
  const kind = auth === 'ok' ? 'write' : 'bad';
  if ((await lib.counted(req, kind)).over) { lib.reply(res, 429, 'too many requests'); return false; }
  if (auth !== 'ok') { lib.reply(res, 401, 'unauthorized'); return false; }
  return true;
}

async function write(req, res) {
  if (!(await writable(req, res))) return;
  const id = lib.idFrom(req);
  if (!lib.ID.test(id)) return lib.reply(res, 400, 'bad id');
  const body = await lib.readBody(req);
  if (body === 'too big') return lib.reply(res, 413, 'too large');
  // Version byte, nonce, at least the tag.
  if (body.length < 1 + 12 + 16 || body[0] !== 0x01) return lib.reply(res, 400, 'bad body');
  await lib.redis([['SET', `s:${id}`, body.toString('base64'), 'EX', lib.settings.ttl]]);
  return lib.reply(res, 204);
}

async function remove(req, res) {
  if (!(await writable(req, res))) return;
  const id = lib.idFrom(req);
  // An id that can't exist is already absent.
  if (lib.ID.test(id)) await lib.redis([['DEL', `s:${id}`]]);
  return lib.reply(res, 204);
}
