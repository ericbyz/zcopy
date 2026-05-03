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
            observer.finishEnumerating(upTo: nil)
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
        service.children(for: .rootContainer) { result in
            switch result {
            case .failure(let error):
                observer.finishEnumeratingWithError(error)
            case .success(let items):
                let previous = self.service.knownItemsSnapshot()
                let current = FileProviderService.signaturesByPath(items)
                let changedItems = items.filter { item in
                    previous[item.path] != current[item.path]
                }
                let deletedIdentifiers = previous.keys
                    .filter { current[$0] == nil }
                    .map { self.service.itemIdentifier(for: $0) }

                if !changedItems.isEmpty {
                    observer.didUpdate(changedItems.map { FileProviderItem(remoteItem: $0, service: self.service) })
                }
                if !deletedIdentifiers.isEmpty {
                    observer.didDeleteItems(withIdentifiers: Array(deletedIdentifiers))
                }
                self.service.replaceKnownItems(items)
                observer.finishEnumeratingChanges(upTo: self.currentAnchor(), moreComing: false)
            }
        }
    }

    func currentSyncAnchor(completionHandler: @escaping (NSFileProviderSyncAnchor?) -> Void) {
        completionHandler(currentAnchor())
    }

    private func currentAnchor() -> NSFileProviderSyncAnchor {
        NSFileProviderSyncAnchor(Data(String(Date().timeIntervalSince1970).utf8))
    }
}
