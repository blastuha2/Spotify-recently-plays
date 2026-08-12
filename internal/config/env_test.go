package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "\ufeff# comment\nSPOTIFY_TEST_ONE=value+with/slash==\nSPOTIFY_TEST_TWO=\"quoted value\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Unsetenv("SPOTIFY_TEST_ONE")
		os.Unsetenv("SPOTIFY_TEST_TWO")
	})
	if err := LoadEnvFile(path); err != nil {
		t.Fatalf("LoadEnvFile: %v", err)
	}
	if got := os.Getenv("SPOTIFY_TEST_ONE"); got != "value+with/slash==" {
		t.Fatalf("first value = %q", got)
	}
	if got := os.Getenv("SPOTIFY_TEST_TWO"); got != "quoted value" {
		t.Fatalf("second value = %q", got)
	}
}

func TestLoadEnvFileDoesNotOverwriteEnvironment(t *testing.T) {
	t.Setenv("SPOTIFY_TEST_EXISTING", "from-environment")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("SPOTIFY_TEST_EXISTING=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("SPOTIFY_TEST_EXISTING"); got != "from-environment" {
		t.Fatalf("existing environment was overwritten: %q", got)
	}
}

func TestLoadEnvFileMissingAndMalformed(t *testing.T) {
	if err := LoadEnvFile(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatalf("missing file: %v", err)
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("not valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err == nil {
		t.Fatal("expected malformed file error")
	}
}
