package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultiServerTokenManager(t *testing.T) {
	_, _ = os.Stat("")
	_, _ = filepath.Abs("")

	t.Run("Set and Get Token", func(t *testing.T) {
		m := NewMultiServerTokenManager("")
		m.SetToken("server1", "token123")
		token, ok := m.GetToken("server1")
		if !ok || token != "token123" {
			t.Fatalf("expected token123, got %s (ok=%v)", token, ok)
		}
	})

	t.Run("Get Non-existent Token", func(t *testing.T) {
		m := NewMultiServerTokenManager("")
		token, ok := m.GetToken("nonexistent")
		if ok || token != "" {
			t.Fatalf("expected empty token and ok=false, got %s (ok=%v)", token, ok)
		}
	})

	t.Run("Remove Token", func(t *testing.T) {
		m := NewMultiServerTokenManager("")
		m.SetToken("server1", "token1")
		m.SetToken("server2", "token2")
		m.RemoveToken("server1")

		token, ok := m.GetToken("server1")
		if ok || token != "" {
			t.Fatalf("server1 token should be removed")
		}

		token, ok = m.GetToken("server2")
		if !ok || token != "token2" {
			t.Fatalf("server2 token should still exist")
		}
	})

	t.Run("Persistence", func(t *testing.T) {
		tempDir := t.TempDir()
		m1 := NewMultiServerTokenManager(tempDir)
		m1.SetToken("serverA", "tokenA")
		m1.SetToken("serverB", "tokenB")
		if err := m1.Save(); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		m2 := NewMultiServerTokenManager(tempDir)
		if err := m2.Load(); err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		token, ok := m2.GetToken("serverA")
		if !ok || token != "tokenA" {
			t.Fatalf("serverA token not loaded correctly")
		}
		token, ok = m2.GetToken("serverB")
		if !ok || token != "tokenB" {
			t.Fatalf("serverB token not loaded correctly")
		}
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		m := NewMultiServerTokenManager("")
		const goroutines = 10
		done := make(chan bool, goroutines)

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()
				serverID := string(rune('a' + id))
				m.SetToken(serverID, "token"+serverID)
				_, _ = m.GetToken(serverID)
			}(i)
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}
	})
}
