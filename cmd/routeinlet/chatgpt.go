package main

import (
	"bytes"
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
	// macOS commonly exposes /var and /tmp through symlinked parents. Store
	// the physical path so checking from either spelling produces the same config.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		_ = os.Remove(root)
		return err
	}
	root = resolvedRoot
	authDir := filepath.Join(root, "auth")
	if err := os.Mkdir(authDir, 0700); err != nil {
		_ = os.Remove(root)
		return err
	}
	configPath := filepath.Join(root, "config.yaml")
	err = os.WriteFile(configPath, pilotConfig(authDir, localKey), 0600)
	if err != nil {
		_ = os.Remove(configPath)
		_ = os.Remove(authDir)
		_ = os.Remove(root)
	}
	return err
}

func pilotConfig(authDir, localKey string) []byte {
	// JSON strings are valid quoted YAML scalars, including for Windows paths.
	key, _ := json.Marshal(localKey)
	path, _ := json.Marshal(authDir)
	return []byte(fmt.Sprintf(chatGPTConfig, key, path))
}

// Check from the connector's working directory, before login or serving.
func checkChatGPTPilot(localKey string) (string, error) {
	for _, entry := range os.Environ() {
		name, value, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if value != "" && (upper == "HOME_JWT" || upper == "WRITABLE_PATH" || upper == "DEPLOY" ||
			strings.HasPrefix(upper, "PGSTORE_") || strings.HasPrefix(upper, "GITSTORE_") || strings.HasPrefix(upper, "OBJECTSTORE_")) {
			return "", fmt.Errorf("unset connector override %s", name)
		}
	}
	if _, err := os.Lstat(".env"); !os.IsNotExist(err) {
		return "", errors.New("pilot working directory must be readable and free of .env entries")
	}
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	authDir := filepath.Join(root, "auth")
	info, err := os.Lstat(authDir)
	if err != nil || !info.IsDir() {
		return "", errors.New("pilot auth directory is missing or is not a real directory")
	}
	info, err = os.Lstat("config.yaml")
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("pilot config must be a regular file")
	}
	config, err := os.ReadFile("config.yaml")
	if err != nil {
		return "", err
	}
	// Comparing the entire generated config also rejects extra stores, duplicate
	// YAML keys, managed configuration, or a connection added to the profile.
	if localKey == "" || !bytes.Equal(config, pilotConfig(authDir, localKey)) {
		return "", errors.New("config differs from the generated profile for this directory and connector key")
	}
	return filepath.EvalSymlinks(authDir)
}
