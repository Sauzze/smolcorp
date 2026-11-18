import Foundation
import Security

/// 32 random bytes, base64url-encoded so they travel in a URL unescaped.
func randomKey() -> String {
    var bytes = [UInt8](repeating: 0, count: 32)
    guard SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes) == errSecSuccess else {
        fatalError("No secure random source available")
    }
    return Data(bytes).base64EncodedString()
        .replacingOccurrences(of: "+", with: "-")
        .replacingOccurrences(of: "/", with: "_")
        .replacingOccurrences(of: "=", with: "")
}

/// Compares without stopping at the first difference, so response timing
/// says nothing about how much of a guess was right.
func constantTimeEqual(_ a: String, _ b: String) -> Bool {
    let x = Array(a.utf8), y = Array(b.utf8)
    guard x.count == y.count else { return false }
    var difference: UInt8 = 0
    for i in x.indices { difference |= x[i] ^ y[i] }
    return difference == 0
}
