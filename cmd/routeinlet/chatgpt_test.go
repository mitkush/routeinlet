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
