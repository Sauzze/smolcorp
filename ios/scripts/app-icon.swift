// The app icon: the portable viewer's black logo (viewer/public/icon-v3-180.png), its 10 × 16 art
// pixels read back cell by cell and drawn at every size the iPhone uses, in whole pixels, centred, opaque, in its two colours.
//
//   swiftc -O ios/scripts/app-icon.swift -o /tmp/app-icon && /tmp/app-icon viewer/public/icon-v3-180.png \
//     ios/Smolcorp/Assets.xcassets/AppIcon.appiconset
import Foundation
import ImageIO
import CoreGraphics
import UniformTypeIdentifiers
let src = CGImageSourceCreateWithURL(URL(fileURLWithPath: CommandLine.arguments[1]) as CFURL, nil)!
let img = CGImageSourceCreateImageAtIndex(src, 0, nil)!
let w = img.width, h = img.height
var data = [UInt8](repeating: 0, count: w*h*4)
let space = CGColorSpace(name: CGColorSpace.sRGB)!
let ctx = CGContext(data: &data, width: w, height: h, bitsPerComponent: 8, bytesPerRow: w*4, space: space, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
ctx.draw(img, in: CGRect(x: 0, y: 0, width: w, height: h))
// The figure: 10 × 16 art pixels of 8 px, from (50, 26), sampled at each cell's centre, with each cell checked to be one colour.
let cols = 10, rows = 16, cell = 8, ox = 50, oy = 26
var grid: [[Bool]] = []
for r in 0..<rows {
    var line: [Bool] = []
    for c in 0..<cols {
        var values = Set<UInt8>()
        for y in 0..<cell { for x in 0..<cell { values.insert(data[((oy + r*cell + y)*w + ox + c*cell + x)*4]) } }
        precondition(values.count == 1, "cell \(c),\(r) is not one colour")
        line.append(values.first! > 128)
    }
    grid.append(line)
}
for line in grid { print(line.map { $0 ? "█" : "·" }.joined()) }
// Each size the iPhone shows, drawn with whole screen pixels per art pixel, so no edge is ever blurred:
// the figure 128/180 of the icon's height, as the viewer's, centred, opaque. 180 is the viewer's own, at 8 px.
let folder = URL(fileURLWithPath: CommandLine.arguments[2], isDirectory: true)
for size in [40, 58, 60, 80, 87, 120, 180, 1024] {
    let scale = max(1, Int(Double(size) * 128 / 180 / Double(rows) + 1e-9))
    let x0 = (size - cols*scale)/2, y0 = (size - rows*scale)/2
    let out = CGContext(data: nil, width: size, height: size, bitsPerComponent: 8, bytesPerRow: 0, space: space, bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue)!
    out.setFillColor(CGColor(srgbRed: 0x0e/255, green: 0x0e/255, blue: 0x0e/255, alpha: 1))
    out.fill(CGRect(x: 0, y: 0, width: size, height: size))
    out.setFillColor(CGColor(srgbRed: 0xf4/255, green: 0xf1/255, blue: 0xea/255, alpha: 1))
    for (r, line) in grid.enumerated() { for (c, on) in line.enumerated() where on {
        out.fill(CGRect(x: x0 + c*scale, y: size - (y0 + (r+1)*scale), width: scale, height: scale))
    } }
    let file = folder.appendingPathComponent("icon-\(size).png")
    let dest = CGImageDestinationCreateWithURL(file as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(dest, out.makeImage()!, nil)
    CGImageDestinationFinalize(dest)
    print("\(size): \(scale) px per art pixel, at \(x0), \(y0)")
}
