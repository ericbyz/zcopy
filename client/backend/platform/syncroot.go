package platform

import (
	"strings"
	"unicode/utf8"
)

const maxCloudFolderDisplayNameRunes = 26

func SyncRootID(taskID string) string {
	var builder strings.Builder
	builder.WriteString("ZCopy.")
	for _, r := range taskID {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return builder.String()
}

func CloudFolderDisplayName(taskName string) string {
	name := strings.TrimSpace(taskName)
	if name == "" {
		return "zcopy"
	}
	if utf8.RuneCountInString(name) <= maxCloudFolderDisplayNameRunes {
		return name
	}

	runes := []rune(name)
	return string(runes[:maxCloudFolderDisplayNameRunes-3]) + "..."
}

func SafeName(name string) string {
	var builder strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		case r == ' ':
			builder.WriteRune('_')
		default:
			builder.WriteRune('-')
		}
	}
	return builder.String()
}
