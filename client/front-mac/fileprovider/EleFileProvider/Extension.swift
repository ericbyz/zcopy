import FileProvider
import Foundation

public final class Extension: NSObject, NSFileProviderReplicatedExtension {
    private let service: FileProviderService
    private let manager: NSFileProviderManager?

    required public init(domain: NSFileProviderDomain) {
        self.service = FileProviderService(domain: domain)
        self.manager = NSFileProviderManager(for: domain)
        super.init()
    }

    public func invalidate() {}

    public func item(for identifier: NSFileProviderItemIdentifier, request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 100)
        service.metadata(for: identifier) { result in
            progress.completedUnitCount = progress.totalUnitCount
            switch result {
            case .failure(let error):
                completionHandler(nil, error)
            case .success(let item):
                completionHandler(FileProviderItem(remoteItem: item, service: self.service), nil)
            }
        }
        progress.cancellationHandler = {
            completionHandler(nil, NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
        }
        return progress
    }

    public func fetchContents(for itemIdentifier: NSFileProviderItemIdentifier, version requestedVersion: NSFileProviderItemVersion?, request: NSFileProviderRequest, completionHandler: @escaping (URL?, NSFileProviderItem?, Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 100)
        service.download(identifier: itemIdentifier) { result in
            progress.completedUnitCount = progress.totalUnitCount
            switch result {
            case .failure(let error):
                completionHandler(nil, nil, error)
            case .success(let (url, item)):
                self.service.markDownloaded(path: item.path)
                self.signalStateRefresh()
                completionHandler(url, FileProviderItem(remoteItem: item, service: self.service), nil)
            }
        }
        progress.cancellationHandler = {
            completionHandler(nil, nil, NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
        }
        return progress
    }

    public func createItem(basedOn itemTemplate: NSFileProviderItem, fields: NSFileProviderItemFields, contents url: URL?, options: NSFileProviderCreateItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 100)
        if service.shouldIgnoreSystemItem(named: itemTemplate.filename) {
            completionHandler(nil, [], false, nil)
            return progress
        }
        let itemPath = service.joined(parent: itemTemplate.parentItemIdentifier, name: itemTemplate.filename)
        let finish: (Result<RemoteFileProviderItem, Error>) -> Void = { result in
            progress.completedUnitCount = progress.totalUnitCount
            switch result {
            case .failure(let error):
                completionHandler(nil, [], false, error)
            case .success(let item):
                self.service.remember(identifier: itemTemplate.itemIdentifier, path: item.path)
                self.service.markDownloaded(path: item.path)
                self.signalStateRefresh()
                completionHandler(FileProviderItem(remoteItem: item, service: self.service), [], false, nil)
            }
        }

        if let url {
            service.upload(contents: url, to: itemPath, completion: finish)
        } else {
            service.createFolder(at: itemPath, completion: finish)
        }
        progress.cancellationHandler = {
            completionHandler(nil, [], false, NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
        }
        return progress
    }

    public func modifyItem(_ item: NSFileProviderItem, baseVersion version: NSFileProviderItemVersion, changedFields: NSFileProviderItemFields, contents newContents: URL?, options: NSFileProviderModifyItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, NSFileProviderItemFields, Bool, Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 100)
        if service.shouldIgnoreSystemItem(named: item.filename) {
            completionHandler(nil, [], false, nil)
            return progress
        }
        let currentPath = service.relativePath(for: item.itemIdentifier)
        let targetPath = service.joined(parent: item.parentItemIdentifier, name: item.filename)
        let renameFields: NSFileProviderItemFields = [.filename, .parentItemIdentifier]

        if changedFields.contains(.parentItemIdentifier), service.isTrashContainer(item.parentItemIdentifier) {
            service.delete(itemPath: currentPath) { result in
                progress.completedUnitCount = progress.totalUnitCount
                switch result {
                case .failure(let error):
                    completionHandler(nil, changedFields, false, error)
                case .success:
                    self.signalStateRefresh()
                    completionHandler(nil, [], false, nil)
                }
            }
            return progress
        }

        let handleContentUpdate: (RemoteFileProviderItem) -> Void = { renamedItem in
            self.service.remember(identifier: item.itemIdentifier, path: renamedItem.path)
            guard changedFields.contains(.contents), let newContents else {
                self.signalStateRefresh()
                progress.completedUnitCount = progress.totalUnitCount
                completionHandler(FileProviderItem(remoteItem: renamedItem, service: self.service), [], false, nil)
                return
            }
            if self.service.isRecentDuplicateUpload(fileURL: newContents, path: renamedItem.path) {
                self.service.markDownloaded(path: renamedItem.path)
                self.signalStateRefresh()
                progress.completedUnitCount = progress.totalUnitCount
                completionHandler(FileProviderItem(remoteItem: renamedItem, service: self.service), [], false, nil)
                return
            }
            self.service.upload(contents: newContents, to: renamedItem.path) { result in
                progress.completedUnitCount = progress.totalUnitCount
                switch result {
                case .failure(let error):
                    completionHandler(nil, [.contents], false, error)
                case .success(let remoteItem):
                    self.service.markDownloaded(path: remoteItem.path)
                    self.signalStateRefresh()
                    completionHandler(FileProviderItem(remoteItem: remoteItem, service: self.service), [], false, nil)
                }
            }
        }

        if !changedFields.intersection(renameFields).isEmpty, currentPath != targetPath {
            service.rename(itemPath: currentPath, to: targetPath) { result in
                switch result {
                case .failure(let error):
                    progress.completedUnitCount = progress.totalUnitCount
                    completionHandler(nil, changedFields, false, error)
                case .success(let renamedItem):
                    handleContentUpdate(renamedItem)
                }
            }
            return progress
        }

        guard changedFields.contains(.contents), let newContents else {
            service.metadata(for: item.itemIdentifier) { result in
                progress.completedUnitCount = progress.totalUnitCount
                switch result {
                case .failure(let error):
                    completionHandler(nil, changedFields, false, error)
                case .success(let remoteItem):
                    completionHandler(FileProviderItem(remoteItem: remoteItem, service: self.service), [], false, nil)
                }
            }
            return progress
        }
        if service.isRecentDuplicateUpload(fileURL: newContents, path: currentPath) {
            service.metadata(for: item.itemIdentifier) { result in
                progress.completedUnitCount = progress.totalUnitCount
                switch result {
                case .failure(let error):
                    completionHandler(nil, [.contents], false, error)
                case .success(let remoteItem):
                    self.service.markDownloaded(path: remoteItem.path)
                    self.signalStateRefresh()
                    completionHandler(FileProviderItem(remoteItem: remoteItem, service: self.service), [], false, nil)
                }
            }
            return progress
        }
        service.upload(contents: newContents, to: service.relativePath(for: item.itemIdentifier)) { result in
            progress.completedUnitCount = progress.totalUnitCount
            switch result {
            case .failure(let error):
                completionHandler(nil, [.contents], false, error)
            case .success(let remoteItem):
                self.service.markDownloaded(path: remoteItem.path)
                self.signalStateRefresh()
                completionHandler(FileProviderItem(remoteItem: remoteItem, service: self.service), [], false, nil)
            }
        }
        progress.cancellationHandler = {
            completionHandler(nil, [.contents], false, NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
        }
        return progress
    }

    public func deleteItem(identifier: NSFileProviderItemIdentifier, baseVersion version: NSFileProviderItemVersion, options: NSFileProviderDeleteItemOptions = [], request: NSFileProviderRequest, completionHandler: @escaping (Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 100)
        service.delete(itemPath: service.relativePath(for: identifier)) { result in
            progress.completedUnitCount = progress.totalUnitCount
            switch result {
            case .failure(let error):
                completionHandler(error)
            case .success:
                self.signalStateRefresh()
                completionHandler(nil)
            }
        }
        progress.cancellationHandler = {
            completionHandler(NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
        }
        return progress
    }

    public func enumerator(for containerItemIdentifier: NSFileProviderItemIdentifier, request: NSFileProviderRequest) throws -> NSFileProviderEnumerator {
        FileProviderEnumerator(service: service, containerItemIdentifier: containerItemIdentifier)
    }

    public func materializedItemsDidChange(completionHandler: @escaping () -> Void) {
        refreshMaterializedState {
            completionHandler()
        }
    }

    private func refreshMaterializedState(completion: @escaping () -> Void) {
        guard let enumerator = manager?.enumeratorForMaterializedItems() else {
            completion()
            return
        }
        let observer = MaterializedSetObserver(service: service, completion: completion)
        enumerator.enumerateItems(for: observer, startingAt: NSFileProviderPage.initialPageSortedByName as NSFileProviderPage)
    }

    private func signalStateRefresh() {
        manager?.signalEnumerator(for: .workingSet) { _ in }
        manager?.signalEnumerator(for: .rootContainer) { _ in }
    }
}

private final class MaterializedSetObserver: NSObject, NSFileProviderEnumerationObserver {
    private let service: FileProviderService
    private let completion: () -> Void
    private var paths = Set<String>()
    private var finished = false

    init(service: FileProviderService, completion: @escaping () -> Void) {
        self.service = service
        self.completion = completion
        super.init()
    }

    func didEnumerate(_ updatedItems: [any NSFileProviderItem]) {
        for item in updatedItems {
            let raw = item.itemIdentifier.rawValue
            if raw.hasPrefix("path:") {
                paths.insert(String(raw.dropFirst(5)))
            }
        }
    }

    func finishEnumerating(upTo nextPage: NSFileProviderPage?) {
        guard !finished else { return }
        finished = true
        service.replaceDownloadedPaths(paths)
        completion()
    }

    func finishEnumeratingWithError(_ error: any Error) {
        guard !finished else { return }
        finished = true
        completion()
    }
}
