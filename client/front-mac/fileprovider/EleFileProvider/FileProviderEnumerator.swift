import FileProvider
import Foundation

final class FileProviderEnumerator: NSObject, NSFileProviderEnumerator {
    private let service: FileProviderService
    private let containerItemIdentifier: NSFileProviderItemIdentifier

    init(service: FileProviderService, containerItemIdentifier: NSFileProviderItemIdentifier) {
        self.service = service
        self.containerItemIdentifier = containerItemIdentifier
        super.init()
    }

    func invalidate() {}

    func enumerateItems(for observer: NSFileProviderEnumerationObserver, startingAt page: NSFileProviderPage) {
        if containerItemIdentifier == .workingSet {
            enumerateWorkingSet(observer: observer)
            return
        }
        service.children(for: containerItemIdentifier) { result in
            switch result {
            case .failure(let error):
                observer.finishEnumeratingWithError(error)
            case .success(let items):
                observer.didEnumerate(items.map { FileProviderItem(remoteItem: $0, service: self.service) })
                observer.finishEnumerating(upTo: nil)
            }
        }
    }

    func enumerateChanges(for observer: NSFileProviderChangeObserver, from syncAnchor: NSFileProviderSyncAnchor) {
        observer.finishEnumeratingChanges(upTo: currentAnchor(), moreComing: false)
    }

    func currentSyncAnchor(completionHandler: @escaping (NSFileProviderSyncAnchor?) -> Void) {
        completionHandler(currentAnchor())
    }

    private func enumerateWorkingSet(observer: NSFileProviderEnumerationObserver) {
        collectItemsRecursively(startingAt: .rootContainer, accumulator: []) { result in
            switch result {
            case .failure(let error):
                observer.finishEnumeratingWithError(error)
            case .success(let items):
                observer.didEnumerate(items.map { FileProviderItem(remoteItem: $0, service: self.service) })
                observer.finishEnumerating(upTo: nil)
            }
        }
    }

    private func collectItemsRecursively(startingAt identifier: NSFileProviderItemIdentifier, accumulator: [RemoteFileProviderItem], completion: @escaping (Result<[RemoteFileProviderItem], Error>) -> Void) {
        service.children(for: identifier) { result in
            switch result {
            case .failure(let error):
                completion(.failure(error))
            case .success(let items):
                let directories = items.filter(\.isDirectory)
                self.collectDirectories(directories, index: 0, accumulator: accumulator + items, completion: completion)
            }
        }
    }

    private func collectDirectories(_ directories: [RemoteFileProviderItem], index: Int, accumulator: [RemoteFileProviderItem], completion: @escaping (Result<[RemoteFileProviderItem], Error>) -> Void) {
        if index >= directories.count {
            completion(.success(accumulator))
            return
        }
        let nextDirectory = directories[index]
        let directoryIdentifier = service.itemIdentifier(for: nextDirectory.path)
        collectItemsRecursively(startingAt: directoryIdentifier, accumulator: accumulator) { result in
            switch result {
            case .failure(let error):
                completion(.failure(error))
            case .success(let updatedAccumulator):
                self.collectDirectories(directories, index: index + 1, accumulator: updatedAccumulator, completion: completion)
            }
        }
    }

    private func currentAnchor() -> NSFileProviderSyncAnchor {
        NSFileProviderSyncAnchor(Data(String(Date().timeIntervalSince1970).utf8))
    }
}
