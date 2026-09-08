package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dakalab/tg-cleaner/internal/config"
)

func TestLeaveCommandPassesConfirmationToRunner(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("telegram:\n  app_id: 1\n  app_hash: hash\n  phone: phone\n")
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var called bool
	runner := func(_ context.Context, credentials config.TelegramConfig, confirm bool, _ io.Reader, _ io.Writer) error {
		called = true
		if credentials.AppID != 1 {
			return fmt.Errorf("app ID = %d, want 1", credentials.AppID)
		}
		if !confirm {
			return fmt.Errorf("confirm = false, want true")
		}
		return nil
	}
	command := newRootCommandWithRunner(runner)
	command.SetArgs([]string{"--config", configPath, "leave", "--confirm"})

	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("runner was not called")
	}
}
