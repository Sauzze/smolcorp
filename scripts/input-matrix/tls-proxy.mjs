// A stand-in for `tailscale serve`, on this computer only, for scripts/input-matrix/page.mjs:
// HTTPS on 127.0.0.1 (a throwaway self-signed certificate), passed on to the test copy with
// the headers `tailscale serve` adds, so the copy sees the request as the tailnet's and the
// page as served over HTTPS, where the phone's own mic and typing work. Changes nothing in
// Tailscale.
//
//   node tls-proxy.mjs <copy port> <cert.pem> <key.pem>   prints the port it listens on
import { createServer } from 'node:https';
import { request } from 'node:http';
import { readFileSync } from 'node:fs';

const [port, cert, key] = process.argv.slice(2);
const server = createServer({ cert: readFileSync(cert), key: readFileSync(key) }, (incoming, outgoing) => {
  const headers = { ...incoming.headers, 'x-forwarded-for': '100.101.102.103', 'x-forwarded-proto': 'https', 'tailscale-user-login': 'tester@example.com' };
  const passed = request({ host: '127.0.0.1', port, method: incoming.method, path: incoming.url, headers }, (answer) => {
    outgoing.writeHead(answer.statusCode, answer.headers);
    answer.pipe(outgoing);
  });
  passed.on('error', () => { outgoing.writeHead(502); outgoing.end(); });
  incoming.pipe(passed);
});
server.listen(0, '127.0.0.1', () => console.log(server.address().port));
