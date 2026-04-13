package app

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"handwritten-notes-ocr/internal/assets"
)

func localURL(port string) string {
	return fmt.Sprintf("http://127.0.0.1:%s", port)
}

func embeddedWebFS() (fs.FS, error) {
	return fs.Sub(assets.FS, "web")
}

func prepareOCRScript(cfg Config) (string, error) {
	if cfg.OCRScriptPath != "" {
		return cfg.OCRScriptPath, nil
	}

	content, err := assets.FS.ReadFile("paddle_ocr.py")
	if err != nil {
		return "", fmt.Errorf("read embedded ocr script: %w", err)
	}

	tempDir := filepath.Join(os.TempDir(), "handwritten-notes-ocr")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	scriptPath := filepath.Join(tempDir, "paddle_ocr.py")
	if err := os.WriteFile(scriptPath, content, 0o644); err != nil {
		return "", fmt.Errorf("write embedded ocr script: %w", err)
	}

	return scriptPath, nil
}

func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

func localhostListener(port string) (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:"+port)
}
