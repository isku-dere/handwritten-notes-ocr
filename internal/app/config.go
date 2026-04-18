package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port           string
	MaxUploadMB    int64
	OCRConcurrency int
	OCRScriptPath  string
	PythonBin      string
	OCRLanguage    string
	OpenBrowser    bool
	OnlineAPIURL   string
	OnlineAPIToken string
	QwenAPIKey     string
	QwenBaseURL    string
	QwenModel      string
}

func LoadConfig() Config {
	loadDotEnv()

	return Config{
		Port:           getEnv("PORT", "8080"),
		MaxUploadMB:    15,
		OCRConcurrency: getEnvInt("OCR_CONCURRENCY", 3),
		OCRScriptPath:  os.Getenv("OCR_SCRIPT_PATH"),
		PythonBin:      getPythonBin(),
		OCRLanguage:    getEnv("OCR_LANG", "ch"),
		OpenBrowser:    getEnv("OPEN_BROWSER", "1") != "0",
		OnlineAPIURL:   os.Getenv("OCR_ONLINE_API_URL"),
		OnlineAPIToken: os.Getenv("OCR_ONLINE_API_TOKEN"),
		QwenAPIKey:     os.Getenv("QWEN_API_KEY"),
		QwenBaseURL:    getEnv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		QwenModel:      getEnv("QWEN_MODEL", "qwen-plus-latest"),
	}
}

func loadDotEnv() {
	for _, path := range dotenvCandidates() {
		if err := loadDotEnvFile(path); err == nil {
			return
		}
	}
}

func dotenvCandidates() []string {
	paths := []string{".env"}

	if exePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), ".env"))
	}

	return paths
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseDotEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}

func parseDotEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return "", "", false
	}

	if len(value) >= 2 {
		quote := value[0]
		if (quote == '"' || quote == '\'') && value[len(value)-1] == quote {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, true
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func getPythonBin() string {
	if value := os.Getenv("OCR_PYTHON_BIN"); value != "" {
		return value
	}

	cwd, err := os.Getwd()
	if err == nil {
		venvPython := filepath.Join(cwd, ".venv", "Scripts", "python.exe")
		if _, statErr := os.Stat(venvPython); statErr == nil {
			return venvPython
		}
	}

	return "python"
}
