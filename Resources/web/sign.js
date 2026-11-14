/*
 * Signs a paired phone's requests, so its key never travels (desktop/signed.go, SECWIFI).
 *
 * Over plain http on the Wi-Fi, anyone on the same network can read every request. So the deck keeps its key
 * in its address's fragment, after #, which a browser never sends, and each request carries instead a signature:
 * HMAC-SHA256, with the SHA-256 of the key as its secret, over the method, the path, the query (with the time,
 * ts, and a random nonce, n) and the SHA-256 of the body. The computer takes each one once, within a minute of
 * its own clock. A browser gives a page crypto.subtle only over HTTPS, so SHA-256 is done here, by hand.
 */
const smolcorpSign = (() => {
  const K = new Uint32Array([
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
  ]);
  const ror = (x, n) => (x >>> n) | (x << (32 - n));
  const utf8 = (text) => new TextEncoder().encode(text);

  /** SHA-256 of bytes, as bytes. */
  function sha256(bytes) {
    const size = ((bytes.length + 9 + 63) >> 6) << 6;
    const m = new Uint8Array(size);
    m.set(bytes);
    m[bytes.length] = 0x80;
    const view = new DataView(m.buffer);
    view.setUint32(size - 8, Math.floor(bytes.length / 0x20000000));
    view.setUint32(size - 4, (bytes.length * 8) >>> 0);
    const H = new Uint32Array([0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]);
    const w = new Uint32Array(64);
    for (let at = 0; at < size; at += 64) {
      for (let i = 0; i < 16; i++) w[i] = view.getUint32(at + i * 4);
      for (let i = 16; i < 64; i++) {
        const s0 = ror(w[i - 15], 7) ^ ror(w[i - 15], 18) ^ (w[i - 15] >>> 3);
        const s1 = ror(w[i - 2], 17) ^ ror(w[i - 2], 19) ^ (w[i - 2] >>> 10);
        w[i] = w[i - 16] + s0 + w[i - 7] + s1;
      }
      let [a, b, c, d, e, f, g, h] = H;
      for (let i = 0; i < 64; i++) {
        const t1 = (h + (ror(e, 6) ^ ror(e, 11) ^ ror(e, 25)) + ((e & f) ^ (~e & g)) + K[i] + w[i]) | 0;
        const t2 = ((ror(a, 2) ^ ror(a, 13) ^ ror(a, 22)) + ((a & b) ^ (a & c) ^ (b & c))) | 0;
        h = g; g = f; f = e; e = (d + t1) | 0; d = c; c = b; b = a; a = (t1 + t2) | 0;
      }
      H[0] += a; H[1] += b; H[2] += c; H[3] += d; H[4] += e; H[5] += f; H[6] += g; H[7] += h;
    }
    const out = new Uint8Array(32);
    H.forEach((v, i) => new DataView(out.buffer).setUint32(i * 4, v));
    return out;
  }

  /** HMAC-SHA256 with a secret of at most 64 bytes. */
  function hmac(secret, bytes) {
    const inner = new Uint8Array(64 + bytes.length);
    const outer = new Uint8Array(64 + 32);
    for (let i = 0; i < 64; i++) {
      inner[i] = (secret[i] || 0) ^ 0x36;
      outer[i] = (secret[i] || 0) ^ 0x5c;
    }
    inner.set(bytes, 64);
    outer.set(sha256(inner), 64);
    return sha256(outer);
  }

  const hex = (bytes) => Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');

  const base64url = (bytes) => btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  /** Random bytes, base64url: crypto.getRandomValues works over plain http too. */
  const nonce = (size = 16) => base64url(crypto.getRandomValues(new Uint8Array(size)));

  let skew = 0; // the computer's clock less this phone's, once it has said

  return {
    /** Takes the computer's time (`now`, in ms, from a refusal) for this phone's, should its own be off. */
    setNow(now) {
      if (Number.isFinite(now)) skew = now - Date.now();
    },
    /** A function that gives the signed address of a request: (method, path, params, body) → "/path?…&sig=…". */
    signer(key) {
      const secret = sha256(utf8(key));
      return (method, path, params = {}, body = '') => {
        const query = new URLSearchParams({ ...params, ts: String(Date.now() + skew), n: nonce() }).toString();
        const message = ['smolcorp-v1', method, path, query, hex(sha256(utf8(body)))].join('\n');
        return `${path}?${query}&sig=${hex(hmac(secret, utf8(message)))}`;
      };
    },
    /**
     * Pairing by the QR code: a part of this phone's own, sent as it claims the code's key, and the phone's key,
     * made of both (desktop/access.go's phoneKeyFrom). A photo of the QR code alone is not the key; with the claim read off the Wi-Fi, it is.
     */
    ownPart: () => nonce(32),
    phoneKey: (codeKey, part) => base64url(hmac(utf8(codeKey), utf8(part))),
    sha256, hmac, hex,
  };
})();
