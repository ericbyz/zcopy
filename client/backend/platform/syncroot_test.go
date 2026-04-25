package platform

import "testing"

func TestCloudFolderDisplayNameUsesTaskName(t *testing.T) {
	got := CloudFolderDisplayName(" Photos ")
	want := "Photos"
	if got != want {
		t.Fatalf("CloudFolderDisplayName() = %q, want %q", got, want)
	}
}

func TestCloudFolderDisplayNameTruncatesLongNames(t *testing.T) {
	got := CloudFolderDisplayName("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if len([]rune(got)) != maxCloudFolderDisplayNameRunes {
		t.Fatalf("CloudFolderDisplayName() length = %d, want %d", len([]rune(got)), maxCloudFolderDisplayNameRunes)
	}
	if got != "abcdefghijklmnopqrstuvw..." {
		t.Fatalf("CloudFolderDisplayName() = %q", got)
	}
}

func TestCloudFolderDisplayNameHandlesEmptyTaskName(t *testing.T) {
	got := CloudFolderDisplayName("   ")
	want := "zcopy"
	if got != want {
		t.Fatalf("CloudFolderDisplayName() = %q, want %q", got, want)
	}
}
