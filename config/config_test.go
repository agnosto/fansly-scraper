package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigApplyFileTimestamps(t *testing.T) {
	cfg := CreateDefaultConfig()
	if cfg.Options.ApplyFileTimestamps != false {
		t.Errorf("expected ApplyFileTimestamps to default to false, got %v", cfg.Options.ApplyFileTimestamps)
	}
}

func TestEnsureConfigUpdatedAddMissingField(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Write an old config file without apply_file_timestamps
	oldConfig := `[account]
auth_token = "dummy_token"
user_agent = "dummy_agent"

[options]
save_location = "/tmp/downloads"
`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	err := EnsureConfigUpdated(configPath)
	if err != nil {
		t.Fatalf("EnsureConfigUpdated failed: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Options.ApplyFileTimestamps != false {
		t.Errorf("expected ApplyFileTimestamps to be updated to false, got %v", cfg.Options.ApplyFileTimestamps)
	}
}

func TestMergeConfigs(t *testing.T) {
	existing := CreateDefaultConfig()
	existing.Options.ApplyFileTimestamps = false

	newCfg := CreateDefaultConfig()
	newCfg.Options.ApplyFileTimestamps = true

	merged := MergeConfigs(existing, newCfg)
	if merged.Options.ApplyFileTimestamps != true {
		t.Errorf("expected merged ApplyFileTimestamps to be true, got %v", merged.Options.ApplyFileTimestamps)
	}
}
