// Opens spec v1's test vector with the page's own code (public/viewer-crypto.js) and WebCrypto.
//   node viewer/scripts/check-vector.mjs [path/to/viewer-vector-v1.bin]
// With no file, the vector is sealed here, with Node's own AES-GCM and the spec's key, id and nonce, so it
// must come out byte for byte as the spec says, as the Mac's `--viewer-selftest` seals it.
// Exits 0 and prints "MATCH" when everything agrees with the spec, 1 otherwise.
import { readFileSync } from 'node:fs';
import { createCipheriv, createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const ViewerCrypto = createRequire(import.meta.url)('../public/viewer-crypto.js');
const file = process.argv[2];

const SPEC = {
  key: 'AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8',
  id: 'AAECAwQFBgcICQoLDA0ODw',
  aad: 'smolcorp-viewer:v1:AAECAwQFBgcICQoLDA0ODw',
  length: 1053,
  first32: '01a0a1a2a3a4a5a6a7a8a9aaab9d3a0a0f7ffa2e9d0311a5e9364df9ec409c69',
  tag: 'f6d6e6d8ca15701702d68c22df48ef2e',
  sha256: 'aeb857fedacfd942ddda9f7b1824d43230bd580bf0f1ea25ea35972a88798327',
  plaintext: '{"v":1,"at":1792000000000,"office":{"keys":[],"overflow":0}}'.padEnd(1024, ' '),
};

/** The vector sealed as the spec says: 0x01, the nonce a0…ab, then AES-256-GCM of the padded plaintext and its tag. */
function sealed() {
  const nonce = Buffer.from(Array.from({ length: 12 }, (_, i) => 0xa0 + i));
  const cipher = createCipheriv('aes-256-gcm', Buffer.from(ViewerCrypto.fromBase64url(SPEC.key)), nonce);
  cipher.setAAD(Buffer.from(SPEC.aad, 'utf8'));
  const text = Buffer.concat([cipher.update(SPEC.plaintext, 'utf8'), cipher.final()]);
  return new Uint8Array(Buffer.concat([Buffer.from([0x01]), nonce, text, cipher.getAuthTag()]));
}

const body = file ? new Uint8Array(readFileSync(file)) : sealed();
const hex = (b) => Buffer.from(b).toString('hex');
const checks = [];
const check = (name, ok, detail = '') => { checks.push(ok); console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail ? `: ${detail}` : ''}`); };

check('body length', body.length === SPEC.length, String(body.length));
check('body first 32 bytes', hex(body.subarray(0, 32)) === SPEC.first32, hex(body.subarray(0, 32)));
check('tag', hex(body.subarray(-16)) === SPEC.tag, hex(body.subarray(-16)));
check('body SHA-256', createHash('sha256').update(body).digest('hex') === SPEC.sha256);
check('AAD', new TextDecoder().decode(ViewerCrypto.aadFor(SPEC.id)) === SPEC.aad);

const subtle = globalThis.crypto.subtle;
const key = await ViewerCrypto.importKey(SPEC.key, subtle);
check('key imports (non-extractable)', !!key && key.extractable === false);
const plain = await ViewerCrypto.openBody(body, key, SPEC.id, subtle);
const text = plain && new TextDecoder().decode(plain);
check('decrypts, plaintext equals the spec\'s', text === SPEC.plaintext, plain ? `${plain.length} bytes` : 'no plaintext');
const payload = plain && ViewerCrypto.parsePayload(plain);
check('payload parses: v 1, at 1792000000000', !!payload && payload.v === 1 && payload.at === 1792000000000);

// What must fail, and fail alike.
const tampered = body.slice(); tampered[40] ^= 1;
check('tampered body refused', (await ViewerCrypto.openBody(tampered, key, SPEC.id, subtle)) === null);
check('other id refused (AAD)', (await ViewerCrypto.openBody(body, key, 'AAECAwQFBgcICQoLDA0OEA', subtle)) === null);
const wrong = await ViewerCrypto.importKey('AQECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8', subtle);
check('wrong key refused', (await ViewerCrypto.openBody(body, wrong, SPEC.id, subtle)) === null);
check('malformed key refused', (await ViewerCrypto.importKey('short', subtle)) === null);

const ok = checks.every(Boolean);
console.log(ok ? 'MATCH: the vector opens to the spec\'s plaintext' : 'MISMATCH');
process.exit(ok ? 0 : 1);
