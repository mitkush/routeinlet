package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This config targets the CLIProxyAPI revision pinned in the pilot guide.
const chatGPTConfig = `config-version: 8
server:
  host: "127.0.0.1"
  port: 8318
management:
  disable-control-panel: true
access:
  api-keys: [%s]
oauth:
  auth-dir: %s
routing:
  retry:
    request-retry: 0
    max-retry-credentials: 1
requests:
  streaming:
    bootstrap-retries: 0
upstream:
  codex:
    upstream-request-compression: false
    stream-bootstrap-buffering: false
`

func initChatGPTPilot(dir, localKey string) error {
	if strings.TrimSpace(localKey) == "" || strings.ContainsAny(localKey, "\r\n") {
		return errors.New("set ROUTEINLET_CONNECTOR_KEY to a local connector access key")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// Never reuse an existing profile, including a directory containing Claude credentials.
	if err := os.Mkdir(root, 0700); err != nil {
		return fmt.Errorf("create new pilot directory: %w", err)
	}
	authDir := filepath.Join(root, "auth")
	if err := os.Mkdir(authDir, 0700); err != nil {
		_ = os.Remove(root)
		return err
	}
	// JSON strings are valid quoted YAML scalars, including for Windows paths.
	key, _ := json.Marshal(localKey)
	path, _ := json.Marshal(authDir)
	configPath := filepath.Join(root, "config.yaml")
	err = os.WriteFile(configPath, []byte(fmt.Sprintf(chatGPTConfig, key, path)), 0600)
	if err != nil {
		_ = os.Remove(configPath)
		_ = os.Remove(authDir)
		_ = os.Remove(root)
	}
	return err
}
