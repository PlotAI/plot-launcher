// The Plot Launcher is normally started from a shell, where a plot:// link
// arrives as a plain argument. macOS hands the link to a registered app as an
// Apple Event instead, so this shim receives the URL, starts the launcher with
// it as its only argument, and quits once the launcher has finished.

import AppKit
import Darwin
import Foundation

private let launcherURL = Bundle.main.bundleURL
    .appendingPathComponent("Contents/MacOS/plot-launcher")

private let logURL = FileManager.default.homeDirectoryForCurrentUser
    .appendingPathComponent("Library/Logs/plot-launcher.log")

// O_APPEND keeps the shim's lines and the launcher's output from overwriting
// each other, since both write to the same file.
private func logHandle() -> FileHandle? {
    try? FileManager.default.createDirectory(
        at: logURL.deletingLastPathComponent(),
        withIntermediateDirectories: true
    )
    let descriptor = Darwin.open(logURL.path, O_WRONLY | O_CREAT | O_APPEND, 0o644)
    guard descriptor >= 0 else { return nil }
    return FileHandle(fileDescriptor: descriptor, closeOnDealloc: true)
}

private func appendToLog(_ message: String) {
    guard let handle = logHandle() else { return }
    let line = "[\(ISO8601DateFormatter().string(from: Date()))] \(message)\n"
    handle.write(Data(line.utf8))
}

final class ApplicationDelegate: NSObject, NSApplicationDelegate {
    private var pending: [String] = []
    private var running = false

    func application(_ application: NSApplication, open urls: [URL]) {
        pending.append(contentsOf: urls.map { $0.absoluteString })
        pump()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Opened from the Dock or Finder with no link: nothing to forward.
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) { [weak self] in
            guard let self, !self.running, self.pending.isEmpty else { return }
            appendToLog("started without a link, quitting")
            NSApp.terminate(nil)
        }
    }

    private func pump() {
        guard !running else { return }
        guard !pending.isEmpty else {
            NSApp.terminate(nil)
            return
        }

        let link = pending.removeFirst()
        let process = Process()
        process.executableURL = launcherURL
        process.arguments = [link]
        process.standardOutput = logHandle()
        process.standardError = process.standardOutput

        running = true
        process.terminationHandler = { [weak self] finished in
            DispatchQueue.main.async {
                guard let self else { return }
                if finished.terminationStatus != 0 {
                    appendToLog("plot-launcher \(link) exited with \(finished.terminationStatus)")
                }
                self.running = false
                self.pump()
            }
        }

        do {
            appendToLog("forwarding \(link)")
            try process.run()
        } catch {
            appendToLog("could not start \(launcherURL.path): \(error)")
            running = false
            pump()
        }
    }
}

let application = NSApplication.shared
let delegate = ApplicationDelegate()
application.delegate = delegate
application.setActivationPolicy(.prohibited)
application.run()
