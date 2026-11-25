package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitChatGPTPilot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pilot profile")
	const key = "local:key#1"
	if err := initChatGPTPilot(dir, ""); err == nil {
		t.Fatal("accepted empty connector key")
	}
	if err := initChatGPTPilot(dir, key); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.yaml")
	config, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`api-keys: ["local:key#1"]`, `host: "127.0.0.1"`, "port: 8318", "request-retry: 0", "bootstrap-retries: 0"} {
		if !strings.Contains(string(config), want) {
			t.Errorf("missing setting %q", want)
		}
	}
	files, err := os.ReadDir(filepath.Join(dir, "auth"))
	if err != nil || len(files) != 0 {
		t.Fatalf("expected empty auth directory: %v, %v", files, err)
	}
	if err := initChatGPTPilot(dir, "another-key"); err == nil {
		t.Fatal("overwrote existing profile")
	}
	again, err := os.ReadFile(file)
	if err != nil || string(again) != string(config) {
		t.Fatal("existing profile changed")
	}
}

func TestCheckChatGPTPilot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pilot")
	const key = "test-connector-key"
	if err := initChatGPTPilot(dir, key); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Error(err)
		}
	})
	want, err := filepath.EvalSymlinks(filepath.Join(dir, "auth"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := checkChatGPTPilot(key); err != nil || got != want {
		t.Fatalf("auth dir %q, err %v", got, err)
	}
	for _, name := range []string{"HOME_JWT", "pgstore_dsn", "GITSTORE_GIT_URL", "objectstore_endpoint", "DEPLOY", "writable_path"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "private-value")
			_, err := checkChatGPTPilot(key)
			if err == nil || !strings.Contains(err.Error(), name) || strings.Contains(err.Error(), "private-value") {
				t.Fatal("expected a diagnostic identifying only the override name")
			}
		})
	}
	if err := os.WriteFile(".env", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := checkChatGPTPilot(key); err == nil {
		t.Fatal("accepted .env")
	}
	if err := os.Remove(".env"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config.yaml", pilotConfig(wd, key), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := checkChatGPTPilot(key); err == nil {
		t.Fatal("accepted foreign auth directory")
	}
}

func TestPilotProfileThroughSymlinkedParent(t *testing.T) {
	parent := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(parent, alias); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(alias, "pilot")
	const key = "connector-key"
	if err := initChatGPTPilot(dir, key); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Error(err)
		}
	})
	for _, path := range []string{dir, physical} {
		t.Run(path, func(t *testing.T) {
			if err := os.Chdir(path); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PWD", path)
			if got, err := checkChatGPTPilot(key); err != nil || got != filepath.Join(physical, "auth") {
				t.Fatalf("auth dir %q, err %v", got, err)
			}
		})
	}
}
