/*
 * Opening a viewer body: spec v1 §4. One file for the page (a plain script,
 * as window.ViewerCrypto) and for Node (require'd by
 * scripts/check-vector.mjs), so the test vector checks the very code the
 * page runs.
 *
 *   body = 0x01 ‖ nonce (12) ‖ ciphertext ‖ tag (16), AES-256-GCM,
 *   AAD  = UTF-8 "smolcorp-viewer:v1:<id>"
 *
 * Nothing here logs, and a failure of any kind is the same null.
 */
(function (root) {
  'use strict';

  const ID = /^[A-Za-z0-9_-]{22}$/;
  const KEY = /^[A-Za-z0-9_-]{43}$/;
  const VERSION = 0x01;

  /** base64url without padding → bytes, or null if it isn't exactly that. */
  function fromBase64url(text) {
    if (typeof text !== 'string' || !/^[A-Za-z0-9_-]*$/.test(text) || text.length % 4 === 1) return null;
    let binary;
    try { binary = atob(text.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (text.length % 4)) % 4)); } catch { return null; }
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return bytes;
  }

  function aadFor(id) { return new TextEncoder().encode(`smolcorp-viewer:v1:${id}`); }

  /** The viewer key as a non-extractable AES-GCM decrypt key, or null. */
  async function importKey(text, subtle) {
    if (!KEY.test(text || '')) return null;
    const raw = fromBase64url(text);
    if (!raw || raw.length !== 32) return null;
    try {
      return await subtle.importKey('raw', raw, { name: 'AES-GCM' }, false, ['decrypt']);
    } catch {
      return null;
    } finally {
      raw.fill(0);
    }
  }

  /** The plaintext bytes of a body for this id, or null (wrong key, wrong id, tampered, malformed: all alike). */
  async function openBody(body, key, id, subtle) {
    const bytes = body instanceof Uint8Array ? body : new Uint8Array(body);
    if (!key || !ID.test(id || '') || bytes.length < 1 + 12 + 16 || bytes[0] !== VERSION) return null;
    try {
      const plain = await subtle.decrypt(
        { name: 'AES-GCM', iv: bytes.subarray(1, 13), additionalData: aadFor(id), tagLength: 128 },
        key, bytes.subarray(13));
      return new Uint8Array(plain);
    } catch {
      return null;
    }
  }

  /** The payload, { v: 1, at, office }, from plaintext bytes, or null. */
  function parsePayload(plain) {
    try {
      const value = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(plain));
      if (!value || typeof value !== 'object' || value.v !== 1) return null;
      if (!Number.isSafeInteger(value.at) || value.at <= 0) return null;
      if (!value.office || typeof value.office !== 'object' || Array.isArray(value.office)) return null;
      return value;
    } catch {
      return null;
    }
  }

  const api = Object.freeze({ ID, KEY, fromBase64url, aadFor, importKey, openBody, parsePayload });
  if (typeof module === 'object' && module && module.exports) module.exports = api;
  else root.ViewerCrypto = api;
})(typeof self !== 'undefined' ? self : globalThis);
