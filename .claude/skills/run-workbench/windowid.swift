import CoreGraphics
import Foundation

// Prints "windowNumber ownerName widthxheight" for every on-screen window
// whose owner (process) name contains the case-insensitive argument.
// Used by driver.sh to find a Gio app's window for screencapture -l.
let needle = CommandLine.arguments.count > 1 ? CommandLine.arguments[1].lowercased() : ""
let opts: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
guard let list = CGWindowListCopyWindowInfo(opts, kCGNullWindowID) as? [[String: Any]] else {
    exit(1)
}
for w in list {
    let owner = (w[kCGWindowOwnerName as String] as? String) ?? ""
    guard needle.isEmpty || owner.lowercased().contains(needle) else { continue }
    let num = (w[kCGWindowNumber as String] as? Int) ?? 0
    var size = ""
    if let b = w[kCGWindowBounds as String] as? [String: Any],
       let width = b["Width"] as? Double, let height = b["Height"] as? Double {
        size = "\(Int(width))x\(Int(height))"
    }
    print("\(num) \(owner) \(size)")
}
