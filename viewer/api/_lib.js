'use strict';
/*
 * What the relay's routes share: the settings, the bearer check, the id's
 * form, Redis over Upstash's REST API (plain fetch, no dependencies), the
 * rate limit and the replies.
 *
 * Files in api/ whose names start with "_" are not routes on Vercel, so
 * this one is only ever required by the others (and by the dev relay, which
 * swaps the Redis below for one in memory).
 *
 * Nothing here logs: no bodies, no ids, no headers.
 */
const crypto = require('node:crypto');

const ID = /^[A-Za-z0-9_-]{22}$/;
const MAX_BODY = 256 * 1024;   // 413 above this
// Per IP per minute. Writes are counted only once the bearer is good, so a
// stranger behind the same IP can't use up the Mac's budget; refused writes
// have their own, stricter bucket.
const LIMITS = { read: 60, write: 30, bad: 10 };
// A write secret shorter than this counts as none: the Mac makes 43
// characters; one typed by hand that's short could be guessed.
const MIN_SECRET = 32;

/** Settings; the dev relay replaces some through configure(). */
const settings = {
  ttl: 600,
  secret: () => process.env.VIEWER_WRITE_SECRET || '',
  redis: null, // null: Upstash over REST, from the env vars
};

function configure(next) { Object.assign(settings, next); }

/* ------------------------------------------------------------- replies */

const COMMON_HEADERS = {
  'Cache-Control': 'no-store',
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'no-referrer',
  'X-Robots-Tag': 'noindex, nofollow',
};

function reply(res, status, body, type) {
  for (const [name, value] of Object.entries(COMMON_HEADERS)) res.setHeader(name, value);
  if (body === undefined || body === null || status === 204) {
    res.statusCode = status;
    res.end();
    return;
  }
  const data = Buffer.isBuffer(body) ? body : Buffer.from(String(body), 'utf8');
  res.setHeader('Content-Type', type || 'text/plain; charset=utf-8');
  res.setHeader('Content-Length', String(data.length));
  res.statusCode = status;
  res.end(data);
}

function notAllowed(res, allow) {
  res.setHeader('Allow', allow);
  reply(res, 405, 'method not allowed');
}

/* ---------------------------------------------------------------- auth */

/** 'ok', 'unset' (no secret configured, or one too short: writes are 503, never allowed) or 'denied'. */
function authorize(req) {
  const secret = settings.secret();
  if (secret.length < MIN_SECRET) return 'unset';
  const header = String(req.headers.authorization || '');
  const given = header.startsWith('Bearer ') ? header.slice(7) : '';
  // Hashed first, so both sides are the same length whatever was sent,
  // then compared in constant time.
  const a = crypto.createHash('sha256').update(given, 'utf8').digest();
  const b = crypto.createHash('sha256').update(secret, 'utf8').digest();
  return crypto.timingSafeEqual(a, b) && given.length > 0 ? 'ok' : 'denied';
}

/* --------------------------------------------------------------- redis */

function upstash() {
  const url = process.env.KV_REST_API_URL || process.env.UPSTASH_REDIS_REST_URL || '';
  const token = process.env.KV_REST_API_TOKEN || process.env.UPSTASH_REDIS_REST_TOKEN || '';
  return url && token ? { url: url.replace(/\/+$/, ''), token } : null;
}

function redisConfigured() { return !!settings.redis || !!upstash(); }

/**
 * Runs Redis commands in one round trip, through Upstash's pipeline endpoint:
 * resolves to each command's result, in order; rejects on any error.
 */
async function redis(commands) {
  if (settings.redis) return settings.redis(commands);
  const conf = upstash();
  if (!conf) throw new Error('redis not configured');
  const response = await fetch(`${conf.url}/pipeline`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${conf.token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify(commands),
    redirect: 'error',
    signal: AbortSignal.timeout(8000),
  });
  if (!response.ok) throw new Error(`redis http ${response.status}`);
  const results = await response.json();
  if (!Array.isArray(results)) throw new Error('redis reply');
  return results.map((r) => {
    if (r && r.error) throw new Error('redis command');
    return r ? r.result : null;
  });
}

/* ---------------------------------------------------------- rate limit */

function clientIP(req) {
  const real = String(req.headers['x-real-ip'] || '').trim();
  if (real) return real;
  const forwarded = String(req.headers['x-forwarded-for'] || '').split(',')[0].trim();
  return forwarded || (req.socket && req.socket.remoteAddress) || 'unknown';
}

/**
 * Counts this request in its kind's bucket, rl:<kind>:<ip>:<minute>, in one
 * round trip with `more` commands: resolves to { over, results }, the
 * results of `more`. Each request costs one INCR; the EXPIRE that lets the
 * bucket go is sent only with a minute's first, so strangers can't run up
 * the Redis bill as fast.
 */
async function counted(req, kind, more = []) {
  const key = `rl:${kind}:${String(clientIP(req)).slice(0, 64)}:${Math.floor(Date.now() / 60000)}`;
  const [count, ...results] = await redis([['INCR', key], ...more]);
  if (Number(count) === 1) await redis([['EXPIRE', key, 120]]);
  return { over: Number(count) > LIMITS[kind], results };
}

/* ---------------------------------------------------------------- misc */

/** The id from the path, /api/s/<id>, whatever the query says. */
function idFrom(req) {
  const path = String(req.url || '').split('?')[0];
  const last = path.split('/').filter(Boolean).pop() || '';
  try { return decodeURIComponent(last); } catch { return ''; }
}

/**
 * The request's body, at most MAX_BODY bytes: a Buffer, or 'too big'. Read
 * from the stream itself; Vercel's `req.body` helper is lazy and never
 * touched, since it would turn a text body into a string.
 */
async function readBody(req) {
  const length = Number(req.headers['content-length'] || 0);
  if (length > MAX_BODY) return 'too big';
  const chunks = [];
  let size = 0;
  for await (const chunk of req) {
    size += chunk.length;
    if (size > MAX_BODY) return 'too big';
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}

module.exports = {
  ID, MAX_BODY, LIMITS, MIN_SECRET, settings, configure, reply, notAllowed, authorize,
  redis, redisConfigured, counted, idFrom, readBody,
};
