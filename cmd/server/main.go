package main

import (
	"log"
	"net/http"

	"handwritten-notes-ocr/internal/app"
)

func main() {
	cfg := app.LoadConfig()

	server, listener, err := app.NewServer(cfg)
	if err != nil {
		log.Fatalf("build server: %v", err)
	}

	url := "http://127.0.0.1:" + cfg.Port
	log.Printf("listening on %s", url)

	if cfg.OpenBrowser {
		if err := app.OpenBrowser(url); err != nil {
			log.Printf("open browser: %v", err)
		}
	}

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
