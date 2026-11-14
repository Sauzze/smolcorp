import JavaScriptCore
import XCTest

/// Every script the pages run parses. A syntax error drops a whole <script> block in the browser, and the
/// page tests, which read the source as text, would still pass: 1.24.3's build 38 shipped a setup page whose
/// main script declared one name twice (`const forGm`), so most of Setup never ran. This reads each inline
/// script of the pages in Resources/web, and the .js files there, and checks its syntax with JavaScriptCore.
final class WebScriptsParseTests: XCTestCase {
    private let web = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
        .deletingLastPathComponent().appendingPathComponent("Resources/web")

    /// The inline scripts of a page: every <script> without src, but its JSON-LD.
    private func inlineScripts(_ page: String) -> [String] {
        let pattern = try! NSRegularExpression(pattern: "<script(?![^>]*\\bsrc=)(?![^>]*application/ld\\+json)[^>]*>(.*?)</script>",
                                               options: [.dotMatchesLineSeparators])
        let range = NSRange(page.startIndex..., in: page)
        return pattern.matches(in: page, range: range).compactMap { Range($0.range(at: 1), in: page).map { String(page[$0]) } }
    }

    /// nil when it parses, else JavaScriptCore's message and line.
    private func syntaxError(_ script: String) -> String? {
        let context = JSContext()!
        let source = JSStringCreateWithUTF8CString(script)
        defer { JSStringRelease(source) }
        var exception: JSValueRef?
        if JSCheckScriptSyntax(context.jsGlobalContextRef, source, nil, 1, &exception) { return nil }
        let error = JSValue(jsValueRef: exception, in: context)
        return "\(error?.toString() ?? "a syntax error") (line \(error?.forProperty("line")?.toString() ?? "?"))"
    }

    func testEveryPageScriptParses() throws {
        let files = try FileManager.default.contentsOfDirectory(at: web, includingPropertiesForKeys: nil)
        let pages = files.filter { $0.pathExtension == "html" }
        XCTAssertTrue(pages.contains { $0.lastPathComponent == "setup.html" }, "the setup page is read")
        var checked = 0
        for page in pages {
            for (index, script) in inlineScripts(try String(contentsOf: page, encoding: .utf8)).enumerated() {
                checked += 1
                if let error = syntaxError(script) { XCTFail("\(page.lastPathComponent), inline script \(index + 1): \(error)") }
            }
        }
        for file in files where file.pathExtension == "js" {
            checked += 1
            if let error = syntaxError(try String(contentsOf: file, encoding: .utf8)) { XCTFail("\(file.lastPathComponent): \(error)") }
        }
        XCTAssertGreaterThan(checked, 3, "the scripts were found")
    }

    /// The check itself catches what build 38 shipped: one name declared twice in a function.
    func testTheCheckCatchesADoubleDeclaration() {
        XCTAssertNotNil(syntaxError("function f() { const a = 1; const a = 2; }"))
        XCTAssertNil(syntaxError("function f() { const a = 1; const b = a?.x ?? 2; return b; }"))
    }
}
