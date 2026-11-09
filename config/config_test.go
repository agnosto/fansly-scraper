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

func TestEnsureConfigUpdatedAddsUISection(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	// Old config without a [ui] section
	oldConfig := `[account]
auth_token = "dummy_token"
user_agent = "dummy_agent"

[options]
save_location = "/tmp/downloads"
`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	if err := EnsureConfigUpdated(configPath); err != nil {
		t.Fatalf("EnsureConfigUpdated failed: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	def := CreateDefaultConfig()
	if cfg.UI != def.UI {
		t.Errorf("expected [ui] section to be backfilled with defaults, got %+v", cfg.UI)
	}
}

func TestEnsureConfigUpdatedKeepsCustomUIColors(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	oldConfig := `[account]
auth_token = "dummy_token"
user_agent = "dummy_agent"

[options]
save_location = "/tmp/downloads"

[ui]
help_desc_color = "#ffffff"
`
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	if err := EnsureConfigUpdated(configPath); err != nil {
		t.Fatalf("EnsureConfigUpdated failed: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	def := CreateDefaultConfig()
	if cfg.UI.HelpDescColor != "#ffffff" {
		t.Errorf("expected custom help_desc_color to be preserved, got %q", cfg.UI.HelpDescColor)
	}
	// Missing keys should be backfilled with defaults
	if cfg.UI.HelpKeyColor != def.UI.HelpKeyColor {
		t.Errorf("expected help_key_color to be backfilled with default %q, got %q", def.UI.HelpKeyColor, cfg.UI.HelpKeyColor)
	}
}

func TestMergeConfigsUI(t *testing.T) {
	existing := CreateDefaultConfig()
	existing.UI.HelpDescColor = "#ffffff"

	newCfg := CreateDefaultConfig() // all defaults, non-empty
	newCfg.UI.HelpDescColor = ""

	merged := MergeConfigs(existing, newCfg)
	// new is empty for this field, so the existing custom value must survive
	if merged.UI.HelpDescColor != "#ffffff" {
		t.Errorf("expected merged help_desc_color to keep existing custom value, got %q", merged.UI.HelpDescColor)
	}

	newCfg.UI.SelectedColor = "#123456"
	merged = MergeConfigs(existing, newCfg)
	if merged.UI.SelectedColor != "#123456" {
		t.Errorf("expected merged selected_color to take new value, got %q", merged.UI.SelectedColor)
	}
}
