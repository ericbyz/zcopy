import FileProvider
import Foundation
import UniformTypeIdentifiers

final class FileProviderItem: NSObject, NSFileProviderItem {
    let remoteItem: RemoteFileProviderItem
    private let service: FileProviderService

    init(remoteItem: RemoteFileProviderItem, service: FileProviderService) {
        self.remoteItem = remoteItem
        self.service = service
        super.init()
    }

    var itemIdentifier: NSFileProviderItemIdentifier {
        service.itemIdentifier(for: remoteItem.path)
    }

    var parentItemIdentifier: NSFileProviderItemIdentifier {
        service.parentIdentifier(for: remoteItem.path)
    }

    var filename: String {
        remoteItem.name
    }

    var contentType: UTType {
        if remoteItem.isDirectory {
            return .folder
        }
        let ext = (remoteItem.name as NSString).pathExtension
        if let type = UTType(filenameExtension: ext) {
            return type
        }
        return .data
    }

    var capabilities: NSFileProviderItemCapabilities {
        if remoteItem.isDirectory {
            return [.allowsReading, .allowsContentEnumerating, .allowsAddingSubItems, .allowsTrashing, .allowsDeleting, .allowsRenaming, .allowsEvicting]
        }
        return [.allowsReading, .allowsWriting, .allowsTrashing, .allowsDeleting, .allowsRenaming, .allowsEvicting]
    }

    var documentSize: NSNumber? {
        remoteItem.isDirectory ? nil : NSNumber(value: remoteItem.size)
    }

    var childItemCount: NSNumber? {
        remoteItem.isDirectory ? NSNumber(value: remoteItem.childCount) : nil
    }

    var creationDate: Date? {
        remoteItem.updatedAt
    }

    var contentModificationDate: Date? {
        remoteItem.updatedAt
    }

    var itemVersion: NSFileProviderItemVersion {
        let version = "\(remoteItem.updatedAt.timeIntervalSince1970)-\(remoteItem.size)-\(remoteItem.isDirectory)"
        let data = Data(version.utf8)
        return NSFileProviderItemVersion(contentVersion: data, metadataVersion: data)
    }

    var isUploaded: Bool {
        true
    }

    var isUploading: Bool {
        false
    }

    var uploadingError: Error? {
        nil
    }

    var isDownloaded: Bool {
        remoteItem.isDirectory || service.isDownloaded(path: remoteItem.path)
    }

    var isDownloading: Bool {
        false
    }

    var downloadingError: Error? {
        nil
    }

    var isMostRecentVersionDownloaded: Bool {
        isDownloaded
    }

    var contentPolicy: NSFileProviderContentPolicy {
        if remoteItem.isDirectory {
            return .inherited
        }
        return .downloadLazily
    }
}
