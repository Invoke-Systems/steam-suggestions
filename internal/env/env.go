package env

import (
	"bufio"
	"os"
	"strings"
)

func Load(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scan := bufio.NewScanner(file)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		cur, exists := os.LookupEnv(key)
		if exists && strings.TrimSpace(cur) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
	_ = scan.Err()
}

func Get(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func Bool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value != "0" && value != "false" && value != "no"
}
