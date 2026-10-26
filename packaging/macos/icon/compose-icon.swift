// Composes the plot app icon from a rasterised copy of the Plot logo: the glyph
// in white on a rounded square in the brand blue, on the macOS icon grid (824 of
// 1024 with a 185 corner radius).
//
// Rasterisers flatten an SVG onto white, so the glyph's coverage is recovered
// from the render itself. The logo is one path in #1f5f8b over white, which
// makes the blue channel a linear measure of coverage.

import AppKit
import Foundation

private let canvasSide = 1024.0
private let plateSide = 824.0
private let plateRadius = 185.0
private let glyphFraction = 0.68
private let inkBlue = 0x8b / 255.0

private func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data("compose-icon: \(message)\n".utf8))
    exit(1)
}

/// Returns the glyph in white, with its coverage taken from the opaque render.
private func whiteGlyph(from render: NSBitmapImageRep) -> NSBitmapImageRep {
    let width = render.pixelsWide
    let height = render.pixelsHigh
    guard let glyph = NSBitmapImageRep(
        bitmapDataPlanes: nil,
        pixelsWide: width,
        pixelsHigh: height,
        bitsPerSample: 8,
        samplesPerPixel: 4,
        hasAlpha: true,
        isPlanar: false,
        colorSpaceName: .deviceRGB,
        bytesPerRow: 0,
        bitsPerPixel: 0
    ) else {
        fail("could not allocate the glyph bitmap")
    }
    guard let source = render.bitmapData, let target = glyph.bitmapData else {
        fail("could not access bitmap data")
    }

    let sourceRow = render.bytesPerRow
    let sourceStep = render.bitsPerPixel / 8
    let targetRow = glyph.bytesPerRow
    let targetStep = glyph.bitsPerPixel / 8

    for y in 0 ..< height {
        for x in 0 ..< width {
            let sourcePixel = source + y * sourceRow + x * sourceStep
            let coverage = (1.0 - Double(sourcePixel[2]) / 255.0) / (1.0 - inkBlue)
            let alpha = UInt8(max(0, min(1, coverage)) * 255)
            // The bitmap is premultiplied, and the glyph is white, so every
            // channel carries the coverage.
            let targetPixel = target + y * targetRow + x * targetStep
            targetPixel[0] = alpha
            targetPixel[1] = alpha
            targetPixel[2] = alpha
            targetPixel[3] = alpha
        }
    }
    return glyph
}

let arguments = CommandLine.arguments
guard arguments.count == 3 else {
    fail("usage: compose-icon <logo.png> <output.png>")
}
guard let logo = NSImage(contentsOfFile: arguments[1]),
      let logoRepresentation = logo.tiffRepresentation,
      let logoBitmap = NSBitmapImageRep(data: logoRepresentation) else {
    fail("could not read \(arguments[1])")
}
let glyph = whiteGlyph(from: logoBitmap)

guard let canvas = NSBitmapImageRep(
    bitmapDataPlanes: nil,
    pixelsWide: Int(canvasSide),
    pixelsHigh: Int(canvasSide),
    bitsPerSample: 8,
    samplesPerPixel: 4,
    hasAlpha: true,
    isPlanar: false,
    colorSpaceName: .deviceRGB,
    bytesPerRow: 0,
    bitsPerPixel: 0
) else {
    fail("could not allocate the canvas bitmap")
}
guard let context = NSGraphicsContext(bitmapImageRep: canvas) else {
    fail("could not create a drawing context")
}

NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = context
context.imageInterpolation = .high

NSRect(x: 0, y: 0, width: canvasSide, height: canvasSide).fill(using: .clear)

let plate = NSRect(
    x: (canvasSide - plateSide) / 2,
    y: (canvasSide - plateSide) / 2,
    width: plateSide,
    height: plateSide
)
NSGraphicsContext.saveGraphicsState()
NSBezierPath(roundedRect: plate, xRadius: plateRadius, yRadius: plateRadius).addClip()
NSColor(srgbRed: 0x1f / 255.0, green: 0x5f / 255.0, blue: inkBlue, alpha: 1).setFill()
plate.fill()
NSGraphicsContext.restoreGraphicsState()

let glyphSide = plateSide * glyphFraction
let glyphRect = CGRect(
    x: (canvasSide - glyphSide) / 2,
    y: (canvasSide - glyphSide) / 2,
    width: glyphSide,
    height: glyphSide
)
guard let glyphImage = glyph.cgImage else {
    fail("could not build a glyph image")
}
context.cgContext.interpolationQuality = .high
context.cgContext.draw(glyphImage, in: glyphRect)

context.flushGraphics()
NSGraphicsContext.restoreGraphicsState()

guard let png = canvas.representation(using: .png, properties: [:]) else {
    fail("could not encode the icon")
}
try png.write(to: URL(fileURLWithPath: arguments[2]))
