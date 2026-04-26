import AppKit
import FileProvider
import Foundation
import Network

struct BridgeInfo: Codable {
    let url: String
    let token: String
}

struct RegisterRequest: Codable {
    let id: String
    let name: String
    let url: String
    let user: String
    let password: String
}

struct SignalRequest: Codable {
    let id: String
    let path: String?
}

struct StatusResponse: Codable {
    let registered: Bool
    let reason: String
    let mountPath: String
    let logPath: String
}

struct ErrorResponse: Codable {
    let message: String
}

final class FileProviderHostApp: NSObject, NSApplicationDelegate {
    private var listener: NWListener?
    private var stateDirectory: URL!
    private var bridgeInfoURL: URL!
    private var logURL: URL!
    private var token: String = ""

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.prohibited)
        do {
            try configure()
            try startBridge()
        } catch {
            fputs("[ZCopyFileProviderHost] \(error)\n", stderr)
            NSApp.terminate(nil)
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        false
    }

    func applicationWillTerminate(_ notification: Notification) {
        listener?.cancel()
        if let bridgeInfoURL {
            try? FileManager.default.removeItem(at: bridgeInfoURL)
        }
    }

    private func configure() throws {
        let environment = ProcessInfo.processInfo.environment
        let stateDirPath = environment["ZCOPY_FILE_PROVIDER_STATE_DIR"] ?? ""
        guard !stateDirPath.isEmpty else {
            throw NSError(domain: "ZCopyFileProviderHost", code: 1, userInfo: [NSLocalizedDescriptionKey: "ZCOPY_FILE_PROVIDER_STATE_DIR 未设置"])
        }
        token = environment["ZCOPY_FILE_PROVIDER_BRIDGE_TOKEN"] ?? UUID().uuidString
        stateDirectory = URL(fileURLWithPath: stateDirPath, isDirectory: true)
        bridgeInfoURL = stateDirectory.appendingPathComponent("bridge.json")
        logURL = stateDirectory.appendingPathComponent("host.log")
        try FileManager.default.createDirectory(at: stateDirectory, withIntermediateDirectories: true)
    }

    private func startBridge() throws {
        let listener = try NWListener(using: .tcp, on: .any)
        self.listener = listener
        listener.newConnectionHandler = { [weak self] connection in
            self?.handle(connection: connection)
        }
        listener.stateUpdateHandler = { [weak self] state in
            guard let self else { return }
            switch state {
            case .ready:
                let port = listener.port?.rawValue ?? 0
                let info = BridgeInfo(url: "http://127.0.0.1:\(port)", token: self.token)
                self.writeBridgeInfo(info)
                self.appendLog("bridge ready on \(info.url)")
            case .failed(let error):
                self.appendLog("listener failed: \(error.localizedDescription)")
                NSApp.terminate(nil)
            default:
                break
            }
        }
        listener.start(queue: .main)
    }

    private func writeBridgeInfo(_ info: BridgeInfo) {
        guard let data = try? JSONEncoder().encode(info) else { return }
        try? data.write(to: bridgeInfoURL, options: [.atomic])
    }

    private func handle(connection: NWConnection) {
        connection.start(queue: .global(qos: .userInitiated))
        connection.receive(minimumIncompleteLength: 1, maximumLength: 64 * 1024) { [weak self] data, _, _, error in
            guard let self else {
                connection.cancel()
                return
            }
            if let error {
                self.appendLog("receive failed: \(error.localizedDescription)")
                connection.cancel()
                return
            }
            let requestData = data ?? Data()
            let response = self.process(requestData)
            connection.send(content: response, completion: .contentProcessed { _ in
                connection.cancel()
            })
        }
    }

    private func process(_ data: Data) -> Data {
        guard let text = String(data: data, encoding: .utf8),
              let headerRange = text.range(of: "\r\n\r\n") else {
            return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "invalid request"))
        }

        let headerText = String(text[..<headerRange.lowerBound])
        let bodyText = String(text[headerRange.upperBound...])
        let headerLines = headerText.components(separatedBy: "\r\n")
        guard let requestLine = headerLines.first else {
            return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "missing request line"))
        }

        let parts = requestLine.components(separatedBy: " ")
        guard parts.count >= 2 else {
            return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "invalid request line"))
        }

        let method = parts[0]
        let rawTarget = parts[1]
        let headers = Dictionary(uniqueKeysWithValues: headerLines.dropFirst().compactMap { line -> (String, String)? in
            guard let idx = line.firstIndex(of: ":") else { return nil }
            let key = line[..<idx].trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
            let value = line[line.index(after: idx)...].trimmingCharacters(in: .whitespacesAndNewlines)
            return (key, value)
        })

        guard headers["authorization"] == "Bearer \(token)" else {
            return makeHTTPResponse(status: 401, payload: ErrorResponse(message: "unauthorized"))
        }

        guard let components = URLComponents(string: "http://127.0.0.1" + rawTarget) else {
            return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "invalid target"))
        }

        switch (method, components.path) {
        case ("POST", "/register"):
            guard let bodyData = bodyText.data(using: .utf8) else {
                return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "invalid body"))
            }
            do {
                let payload = try JSONDecoder().decode(RegisterRequest.self, from: bodyData)
                return register(payload)
            } catch {
                return makeHTTPResponse(status: 400, payload: ErrorResponse(message: error.localizedDescription))
            }

        case ("POST", "/signal"):
            guard let bodyData = bodyText.data(using: .utf8) else {
                return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "invalid body"))
            }
            do {
                let payload = try JSONDecoder().decode(SignalRequest.self, from: bodyData)
                return signal(payload)
            } catch {
                return makeHTTPResponse(status: 400, payload: ErrorResponse(message: error.localizedDescription))
            }

        case ("GET", "/status"):
            let query = Dictionary(uniqueKeysWithValues: (components.queryItems ?? []).map { ($0.name, $0.value ?? "") })
            let id = query["id"] ?? ""
            let name = query["name"] ?? ""
            return queryStatus(id: id, name: name)

        case ("GET", "/health"):
            return makeHTTPResponse(status: 200, payload: ["status": "ok"])

        default:
            return makeHTTPResponse(status: 404, payload: ErrorResponse(message: "not found"))
        }
    }

    private func register(_ payload: RegisterRequest) -> Data {
        guard #available(macOS 11.0, *) else {
            return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "macOS 11.0 及以上才支持 File Provider"))
        }

        let semaphore = DispatchSemaphore(value: 0)
        var responseData = makeHTTPResponse(status: 500, payload: ErrorResponse(message: "unknown error"))

        let domain = NSFileProviderDomain(identifier: NSFileProviderDomainIdentifier(payload.id), displayName: payload.name)
        NSFileProviderManager.add(domain) { [weak self] error in
            guard let self else {
                semaphore.signal()
                return
            }
            if let error {
                self.appendLog("register failed: \(error.localizedDescription)")
                responseData = self.makeHTTPResponse(status: 500, payload: ErrorResponse(message: error.localizedDescription))
                semaphore.signal()
                return
            }
            responseData = self.queryStatus(id: payload.id, name: payload.name)
            semaphore.signal()
        }

        _ = semaphore.wait(timeout: .now() + 15)
        return responseData
    }

    private func signal(_ payload: SignalRequest) -> Data {
        guard #available(macOS 11.0, *) else {
            return makeHTTPResponse(status: 400, payload: ErrorResponse(message: "macOS 11.0 及以上才支持 File Provider"))
        }

        let semaphore = DispatchSemaphore(value: 0)
        var responseData = makeHTTPResponse(status: 500, payload: ErrorResponse(message: "unknown error"))
        let domain = NSFileProviderDomain(identifier: NSFileProviderDomainIdentifier(payload.id), displayName: payload.id)
        guard let manager = NSFileProviderManager(for: domain) else {
            return makeHTTPResponse(status: 500, payload: ErrorResponse(message: "无法创建 File Provider manager"))
        }

        let identifiers = signalIdentifiers(for: payload.path)
        let group = DispatchGroup()
        var signalError: Error?

        for identifier in identifiers {
            group.enter()
            manager.signalEnumerator(for: identifier) { [weak self] error in
                if let error {
                    self?.appendLog("signal \(identifier.rawValue) failed: \(error.localizedDescription)")
                    signalError = signalError ?? error
                } else {
                    self?.appendLog("signal \(identifier.rawValue) for \(payload.id)")
                }
                group.leave()
            }
        }
        group.notify(queue: .main) {
            if let signalError {
                responseData = self.makeHTTPResponse(status: 500, payload: ErrorResponse(message: signalError.localizedDescription))
            } else {
                responseData = self.makeHTTPResponse(status: 200, payload: ["status": "ok"])
            }
            semaphore.signal()
        }

        _ = semaphore.wait(timeout: .now() + 10)
        return responseData
    }

    private func signalIdentifiers(for rawPath: String?) -> [NSFileProviderItemIdentifier] {
        let clean = normalizeFileProviderPath(rawPath ?? "")
        var identifiers: [NSFileProviderItemIdentifier] = [.workingSet, .rootContainer]
        let parent = parentPath(for: clean)
        if !parent.isEmpty {
            identifiers.append(NSFileProviderItemIdentifier("path:\(parent)"))
        }

        var seen = Set<String>()
        return identifiers.filter { identifier in
            if seen.contains(identifier.rawValue) {
                return false
            }
            seen.insert(identifier.rawValue)
            return true
        }
    }

    private func parentPath(for path: String) -> String {
        guard !path.isEmpty else { return "" }
        let parent = (path as NSString).deletingLastPathComponent
        return parent == "." ? "" : normalizeFileProviderPath(parent)
    }

    private func normalizeFileProviderPath(_ raw: String) -> String {
        let parts = raw.split(separator: "/").filter { !$0.isEmpty && $0 != "." }
        var stack: [Substring] = []
        for part in parts {
            if part == ".." {
                if !stack.isEmpty {
                    stack.removeLast()
                }
                continue
            }
            stack.append(part)
        }
        return stack.map(String.init).joined(separator: "/")
    }

    private func queryStatus(id: String, name: String) -> Data {
        guard #available(macOS 11.0, *) else {
            return makeHTTPResponse(status: 200, payload: StatusResponse(registered: false, reason: "macOS 11.0 及以上才支持 File Provider", mountPath: "", logPath: logURL.path))
        }

        let semaphore = DispatchSemaphore(value: 0)
        var payload = StatusResponse(registered: false, reason: "", mountPath: "", logPath: logURL.path)
        let domain = NSFileProviderDomain(identifier: NSFileProviderDomainIdentifier(id), displayName: name)
        guard let manager = NSFileProviderManager(for: domain) else {
            return makeHTTPResponse(status: 200, payload: StatusResponse(registered: false, reason: "无法创建 File Provider manager", mountPath: "", logPath: logURL.path))
        }

        manager.getUserVisibleURL(for: .rootContainer) { url, error in
            if let error {
                payload = StatusResponse(registered: false, reason: error.localizedDescription, mountPath: "", logPath: self.logURL.path)
            } else {
                payload = StatusResponse(registered: url != nil, reason: "", mountPath: url?.path ?? "", logPath: self.logURL.path)
            }
            semaphore.signal()
        }

        _ = semaphore.wait(timeout: .now() + 10)
        return makeHTTPResponse(status: 200, payload: payload)
    }

    private func makeHTTPResponse<T: Encodable>(status: Int, payload: T) -> Data {
        let encoder = JSONEncoder()
        let body = (try? encoder.encode(payload)) ?? Data("{}".utf8)
        let statusText: String
        switch status {
        case 200: statusText = "OK"
        case 400: statusText = "Bad Request"
        case 401: statusText = "Unauthorized"
        case 404: statusText = "Not Found"
        case 500: statusText = "Internal Server Error"
        default: statusText = "OK"
        }
        var text = "HTTP/1.1 \(status) \(statusText)\r\n"
        text += "Content-Type: application/json; charset=utf-8\r\n"
        text += "Content-Length: \(body.count)\r\n"
        text += "Connection: close\r\n\r\n"
        var data = Data(text.utf8)
        data.append(body)
        return data
    }

    private func appendLog(_ message: String) {
        guard let logURL else {
            fputs("[ZCopyFileProviderHost] \(message)\n", stderr)
            return
        }
        let line = "[\(ISO8601DateFormatter().string(from: Date()))] \(message)\n"
        if let data = line.data(using: .utf8) {
            if FileManager.default.fileExists(atPath: logURL.path) {
                if let handle = try? FileHandle(forWritingTo: logURL) {
                    _ = try? handle.seekToEnd()
                    try? handle.write(contentsOf: data)
                    try? handle.close()
                }
            } else {
                try? data.write(to: logURL, options: [.atomic])
            }
        }
    }
}

let application = NSApplication.shared
let delegate = FileProviderHostApp()
application.delegate = delegate
application.setActivationPolicy(.prohibited)
application.run()
