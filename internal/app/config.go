package app

import (
	"fmt"
	"os"
	"path/filepath"
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
