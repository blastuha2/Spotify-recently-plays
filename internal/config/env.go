package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadEnvFile loads KEY=VALUE lines without overwriting variables already set
// in the process environment. A missing file is not an error.
func LoadEnvFile(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || !validEnvKey(key) {
			return fmt.Errorf("parse %s line %d: expected KEY=VALUE", path, lineNumber)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func validEnvKey(key string) bool {
	if key == "" || !isEnvLetter(key[0]) {
		return false
	}
	for index := 1; index < len(key); index++ {
		if !isEnvLetter(key[index]) && (key[index] < '0' || key[index] > '9') {
			return false
		}
	}
	return true
}

func isEnvLetter(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}
