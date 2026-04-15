import FileProvider
import Foundation

struct RemoteFileProviderItem: Codable {
    let identifier: String
    let parentIdentifier: String
    let name: String
    let path: String
    let size: Int64
    let isDirectory: Bool
    let updatedAt: Date
    let childCount: Int
}

private struct RemoteItemEnvelope: Codable {
    let item: RemoteFileProviderItem
}

private struct RemoteRenameEnvelope: Codable {
    let item: RemoteFileProviderItem
}

private struct RemoteChildrenEnvelope: Codable {
    let path: String
    let items: [RemoteFileProviderItem]
}

private struct RemoteErrorEnvelope: Codable {
    let message: String
}

enum FileProviderServiceError: LocalizedError {
    case invalidResponse
    case server(String)
    case unsupported(String)

    var errorDescription: String? {
        switch self {
        case .invalidResponse:
            return "本地 File Provider 服务返回了无效响应"
        case .server(let message):
            return message
        case .unsupported(let message):
            return message
        }
    }
}

final class FileProviderService {
    private let baseURL: URL
    private let session: URLSession
    private let decoder: JSONDecoder
    private let downloadedStateURL: URL
    private let downloadedStateQueue = DispatchQueue(label: "com.zcopy.fileprovider.downloaded-state")
    private var downloadedPaths: Set<String>

    init(domain: NSFileProviderDomain) {
        let taskID = domain.identifier.rawValue.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? domain.identifier.rawValue
        self.baseURL = URL(string: "http://127.0.0.1:8090/api/v1/file-provider/tasks/\(taskID)")!
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = 60
        configuration.timeoutIntervalForResource = 300
        self.session = URLSession(configuration: configuration)
        self.decoder = JSONDecoder()
        self.decoder.dateDecodingStrategy = .iso8601
        let supportRoot = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
            ?? FileManager.default.temporaryDirectory
        let stateDirectory = supportRoot.appendingPathComponent("ZCopyFileProvider", isDirectory: true)
        try? FileManager.default.createDirectory(at: stateDirectory, withIntermediateDirectories: true)
        let safeDomainID = domain.identifier.rawValue.replacingOccurrences(of: "/", with: "_")
        self.downloadedStateURL = stateDirectory.appendingPathComponent("\(safeDomainID)-downloaded.json")
        self.downloadedPaths = Self.loadDownloadedPaths(from: downloadedStateURL)
    }

    func metadata(for identifier: NSFileProviderItemIdentifier, completion: @escaping (Result<RemoteFileProviderItem, Error>) -> Void) {
        requestJSON(path: "item", itemPath: relativePath(for: identifier), method: "GET", body: nil) { (result: Result<RemoteItemEnvelope, Error>) in
            completion(result.map(\.item))
        }
    }

    func children(for identifier: NSFileProviderItemIdentifier, completion: @escaping (Result<[RemoteFileProviderItem], Error>) -> Void) {
        requestJSON(path: "children", itemPath: relativePath(for: identifier), method: "GET", body: nil) { (result: Result<RemoteChildrenEnvelope, Error>) in
            completion(result.map(\.items))
        }
    }

    func createFolder(at itemPath: String, completion: @escaping (Result<RemoteFileProviderItem, Error>) -> Void) {
        requestJSON(path: "folder", itemPath: itemPath, method: "POST", body: Data()) { (result: Result<RemoteItemEnvelope, Error>) in
            completion(result.map(\.item))
        }
    }

    func upload(contents fileURL: URL, to itemPath: String, completion: @escaping (Result<RemoteFileProviderItem, Error>) -> Void) {
        do {
            let data = try Data(contentsOf: fileURL)
            requestJSON(path: "content", itemPath: itemPath, method: "PUT", body: data) { (result: Result<RemoteItemEnvelope, Error>) in
                completion(result.map(\.item))
            }
        } catch {
            completion(.failure(error))
        }
    }

    func download(identifier: NSFileProviderItemIdentifier, completion: @escaping (Result<(URL, RemoteFileProviderItem), Error>) -> Void) {
        metadata(for: identifier) { result in
            switch result {
            case .failure(let error):
                completion(.failure(error))
            case .success(let item):
                if item.isDirectory {
                    completion(.failure(FileProviderServiceError.unsupported("目录不支持内容下载")))
                    return
                }
                var request = URLRequest(url: self.makeURL(path: "content", itemPath: item.path))
                request.httpMethod = "GET"
                let task = self.session.downloadTask(with: request) { url, response, error in
                    if let error {
                        completion(.failure(error))
                        return
                    }
                    guard let http = response as? HTTPURLResponse else {
                        completion(.failure(FileProviderServiceError.invalidResponse))
                        return
                    }
                    guard (200 ..< 300).contains(http.statusCode), let url else {
                        completion(.failure(self.errorFromResponseData(nil, statusCode: http.statusCode)))
                        return
                    }
                    let ext = (item.name as NSString).pathExtension
                    let tempURL = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString).appendingPathExtension(ext)
                    do {
                        if FileManager.default.fileExists(atPath: tempURL.path) {
                            try FileManager.default.removeItem(at: tempURL)
                        }
                        try FileManager.default.moveItem(at: url, to: tempURL)
                        completion(.success((tempURL, item)))
                    } catch {
                        completion(.failure(error))
                    }
                }
                task.resume()
            }
        }
    }

    func delete(itemPath: String, completion: @escaping (Result<Void, Error>) -> Void) {
        var request = URLRequest(url: makeURL(path: "item", itemPath: itemPath))
        request.httpMethod = "DELETE"
        let task = session.dataTask(with: request) { data, response, error in
            if let error {
                completion(.failure(error))
                return
            }
            guard let http = response as? HTTPURLResponse else {
                completion(.failure(FileProviderServiceError.invalidResponse))
                return
            }
            guard (200 ..< 300).contains(http.statusCode) else {
                completion(.failure(self.errorFromResponseData(data, statusCode: http.statusCode)))
                return
            }
            self.markEvicted(path: itemPath)
            completion(.success(()))
        }
        task.resume()
    }

    func rename(itemPath: String, to newPath: String, completion: @escaping (Result<RemoteFileProviderItem, Error>) -> Void) {
        let payload = ["newPath": normalizedPath(newPath)]
        guard let body = try? JSONSerialization.data(withJSONObject: payload) else {
            completion(.failure(FileProviderServiceError.invalidResponse))
            return
        }
        requestJSON(path: "rename", itemPath: itemPath, method: "PUT", body: body) { (result: Result<RemoteRenameEnvelope, Error>) in
            completion(result.map(\.item))
        }
    }

    func isDownloaded(path: String) -> Bool {
        let clean = normalizedPath(path)
        if clean.isEmpty {
            return true
        }
        return downloadedStateQueue.sync {
            downloadedPaths.contains(clean)
        }
    }

    func markDownloaded(path: String) {
        let clean = normalizedPath(path)
        guard !clean.isEmpty else { return }
        downloadedStateQueue.sync {
            downloadedPaths.insert(clean)
            persistDownloadedPaths()
        }
    }

    func markEvicted(path: String) {
        let clean = normalizedPath(path)
        downloadedStateQueue.sync {
            if clean.isEmpty {
                downloadedPaths.removeAll()
            } else {
                downloadedPaths = downloadedPaths.filter { $0 != clean && !$0.hasPrefix(clean + "/") }
            }
            persistDownloadedPaths()
        }
    }

    func replaceDownloadedPaths(_ paths: Set<String>) {
        downloadedStateQueue.sync {
            downloadedPaths = Set(paths.map(normalizedPath).filter { !$0.isEmpty })
            persistDownloadedPaths()
        }
    }

    func relativePath(for identifier: NSFileProviderItemIdentifier) -> String {
        if identifier == .rootContainer || identifier == .workingSet || identifier.rawValue == "root" {
            return ""
        }
        if identifier.rawValue.hasPrefix("path:") {
            return normalizedPath(String(identifier.rawValue.dropFirst(5)))
        }
        return normalizedPath(identifier.rawValue)
    }

    func itemIdentifier(for path: String) -> NSFileProviderItemIdentifier {
        let clean = normalizedPath(path)
        if clean.isEmpty {
            return .rootContainer
        }
        return NSFileProviderItemIdentifier("path:\(clean)")
    }

    func parentIdentifier(for path: String) -> NSFileProviderItemIdentifier {
        let clean = normalizedPath(path)
        if clean.isEmpty {
            return .rootContainer
        }
        let parent = normalizedPath((clean as NSString).deletingLastPathComponent)
        return itemIdentifier(for: parent)
    }

    func joined(parent: NSFileProviderItemIdentifier, name: String) -> String {
        let parentPath = relativePath(for: parent)
        if parentPath.isEmpty {
            return normalizedPath(name)
        }
        return normalizedPath(parentPath + "/" + name)
    }

    private func requestJSON<T: Decodable>(path: String, itemPath: String, method: String, body: Data?, completion: @escaping (Result<T, Error>) -> Void) {
        var request = URLRequest(url: makeURL(path: path, itemPath: itemPath))
        request.httpMethod = method
        if let body {
            request.httpBody = body
            if !body.isEmpty {
                request.setValue("application/octet-stream", forHTTPHeaderField: "Content-Type")
            }
        }
        let task = session.dataTask(with: request) { data, response, error in
            if let error {
                completion(.failure(error))
                return
            }
            guard let http = response as? HTTPURLResponse else {
                completion(.failure(FileProviderServiceError.invalidResponse))
                return
            }
            guard (200 ..< 300).contains(http.statusCode), let data else {
                completion(.failure(self.errorFromResponseData(data, statusCode: http.statusCode)))
                return
            }
            do {
                completion(.success(try self.decoder.decode(T.self, from: data)))
            } catch {
                completion(.failure(error))
            }
        }
        task.resume()
    }

    private func makeURL(path: String, itemPath: String) -> URL {
        var components = URLComponents(url: baseURL.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
        if !itemPath.isEmpty {
            components.queryItems = [URLQueryItem(name: "path", value: normalizedPath(itemPath))]
        }
        return components.url!
    }

    private func errorFromResponseData(_ data: Data?, statusCode: Int) -> Error {
        if let data,
           let payload = try? decoder.decode(RemoteErrorEnvelope.self, from: data),
           !payload.message.isEmpty {
            return FileProviderServiceError.server(payload.message)
        }
        return FileProviderServiceError.server("本地 File Provider 服务返回状态码 \(statusCode)")
    }

    private func normalizedPath(_ raw: String) -> String {
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

    private func persistDownloadedPaths() {
        let payload = Array(downloadedPaths).sorted()
        guard let data = try? JSONEncoder().encode(payload) else {
            return
        }
        try? data.write(to: downloadedStateURL, options: [.atomic])
    }

    private static func loadDownloadedPaths(from url: URL) -> Set<String> {
        guard let data = try? Data(contentsOf: url),
              let payload = try? JSONDecoder().decode([String].self, from: data) else {
            return []
        }
        return Set(payload)
    }
}
