import FileProvider
import Foundation

public final class Extension: NSObject, NSFileProviderReplicatedExtension {
    private let service: FileProviderService

    required public init(domain: NSFileProviderDomain) {
        self.service = FileProviderService(domain: domain)
        super.init()
    }

    public func invalidate() {}

    public func item(for identifier: NSFileProviderItemIdentifier, request: NSFileProviderRequest, completionHandler: @escaping (NSFileProviderItem?, Error?) -> Void) -> Progress {
        let progress = Progress(totalUnitCount: 100)
        service.metadata(for: identifier) { result in
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
            switch result {
            case .failure(let error):
                completionHandler(nil, nil, error)
            case .success(let (url, item)):
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
        let itemPath = service.joined(parent: itemTemplate.parentItemIdentifier, name: itemTemplate.filename)
        let finish: (Result<RemoteFileProviderItem, Error>) -> Void = { result in
            switch result {
            case .failure(let error):
                completionHandler(nil, [], false, error)
            case .success(let item):
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
        let unsupportedFields: NSFileProviderItemFields = [.filename, .parentItemIdentifier]
        if !changedFields.intersection(unsupportedFields).isEmpty {
            completionHandler(nil, changedFields, false, FileProviderServiceError.unsupported("暂不支持重命名或移动"))
            return progress
        }
        guard changedFields.contains(.contents), let newContents else {
            service.metadata(for: item.itemIdentifier) { result in
                switch result {
                case .failure(let error):
                    completionHandler(nil, changedFields, false, error)
                case .success(let remoteItem):
                    completionHandler(FileProviderItem(remoteItem: remoteItem, service: self.service), [], false, nil)
                }
            }
            return progress
        }
        service.upload(contents: newContents, to: service.relativePath(for: item.itemIdentifier)) { result in
            switch result {
            case .failure(let error):
                completionHandler(nil, [.contents], false, error)
            case .success(let remoteItem):
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
            switch result {
            case .failure(let error):
                completionHandler(error)
            case .success:
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
}
