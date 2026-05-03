package serverinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestGenerateUUID(t *testing.T) {
	uuid, err := generateUUID()
	if err != nil {
		t.Fatalf("generateUUID failed: %v", err)
	}

	if uuid == "" {
		t.Error("uuid is empty")
	}

	if len(uuid) != 36 {
		t.Errorf("uuid length is %d, expected 36", len(uuid))
	}

	if !uuidRegex.MatchString(uuid) {
		t.Errorf("uuid %q does not match v4 regex", uuid)
	}
}

func TestInitServerInfo(t *testing.T) {
	tempDir := t.TempDir()

	info1, err := InitServerInfo(tempDir, "localhost:8890")
	if err != nil {
		t.Fatalf("first InitServerInfo failed: %v", err)
	}

	if info1.UUID == "" {
		t.Error("info1.UUID is empty")
	}

	if !uuidRegex.MatchString(info1.UUID) {
		t.Errorf("info1.UUID %q does not match v4 regex", info1.UUID)
	}

	if info1.Name != "ZCopy Server" {
		t.Errorf("info1.Name is %q, expected %q", info1.Name, "ZCopy Server")
	}

	if info1.Version != "1.0.0" {
		t.Errorf("info1.Version is %q, expected %q", info1.Version, "1.0.0")
	}

	if info1.Address != "localhost:8890" {
		t.Errorf("info1.Address is %q, expected %q", info1.Address, "localhost:8890")
	}

	filePath := filepath.Join(tempDir, "server_info.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("server_info.json not created")
	}

	info2, err := InitServerInfo(tempDir, "localhost:9999")
	if err != nil {
		t.Fatalf("second InitServerInfo failed: %v", err)
	}

	if info2.UUID != info1.UUID {
		t.Errorf("info2.UUID %q != info1.UUID %q", info2.UUID, info1.UUID)
	}

	info3 := GetServerInfo()
	if info3.UUID != info1.UUID {
		t.Errorf("info3.UUID %q != info1.UUID %q", info3.UUID, info1.UUID)
	}

	instance = nil

	info4, err := InitServerInfo(tempDir, "localhost:9999")
	if err != nil {
		t.Fatalf("third InitServerInfo failed: %v", err)
	}

	if info4.UUID != info1.UUID {
		t.Errorf("info4.UUID %q != info1.UUID %q", info4.UUID, info1.UUID)
	}
}
