# Handwritten Notes OCR

[中文](./README.md)

An offline-first handwritten note OCR application built with `Go + PaddleOCR + embedded web UI`, with optional online OCR fallback and Qwen-based study note generation.

It is designed for turning photographed handwritten notes into editable Markdown, then refining and merging them into a structured electronic study note.

## Features

- Batch upload and batch OCR processing
- Incremental queue updates during concurrent OCR
- Per-image Markdown result retention and editing
- Markdown source / preview modes
- Image rotation before OCR
- Soft remove / restore for queue items
- Local Markdown export for single-note and study-note outputs
- Study note generation from all recognized Markdown on the page
- Precheck before LLM generation to avoid unnecessary token usage on short content
- Embedded frontend assets served by the Go binary

## Tech Stack

- Backend: Go
- OCR:
  - Local: PaddleOCR via Python
  - Optional fallback/primary online OCR endpoint
- LLM: Qwen via OpenAI-compatible API
- Frontend: HTML, CSS, vanilla JavaScript
- Transport:
  - REST for upload and control endpoints
  - SSE streaming for batch OCR updates and study-note generation

## How It Works

1. Upload one or more note images.
2. The frontend can rotate images before submission.
3. The Go backend processes OCR requests concurrently.
4. Each image produces an individual Markdown result that can be edited manually.
5. The app collects all non-removed recognized Markdown entries.
6. Before calling the LLM, the backend performs a short-content precheck.
7. If the content is sufficient, Qwen generates a corrected and merged study note in Markdown.

## Project Structure

```text
cmd/server/                 Go entrypoint
internal/app/               server, routes, config, runtime
internal/ocr/               OCR client
internal/notes/             study-note prompt and precheck logic
internal/llm/               Qwen client
internal/markdown/          OCR-to-Markdown builder
internal/assets/            embedded assets and OCR helper script
internal/assets/web/        frontend UI
```

## Run Locally

### Requirements

- Go `1.25+`
- Python `3.10+`
- Windows environment tested

### Install Python dependencies

```bash
pip install -r requirements.txt
```

### Start from source

```bash
go run ./cmd/server
```

### Build executable

```bash
go build -o handwritten-notes-ocr.exe ./cmd/server
```

By default, the app opens at:

[http://127.0.0.1:8080](http://127.0.0.1:8080)

## Environment Variables

Use local environment variables or a local `.env` file kept out of Git.

See `.env.example` for a template.

- `PORT`: local server port, default `8080`
- `OPEN_BROWSER`: open browser on startup, default `1`
- `OCR_PYTHON_BIN`: Python executable path
- `OCR_SCRIPT_PATH`: optional external OCR script path
- `OCR_LANG`: OCR language, default `ch`
- `OCR_CONCURRENCY`: backend OCR concurrency, default `3`
- `OCR_ONLINE_API_URL`: optional online OCR endpoint
- `OCR_ONLINE_API_TOKEN`: online OCR token
- `QWEN_API_KEY`: Qwen API key
- `QWEN_BASE_URL`: Qwen-compatible API base URL
- `QWEN_MODEL`: Qwen model name

## API Overview

### Health

- `GET /api/health`

### OCR

- `POST /api/ocr`
- `POST /api/ocr/batch`
- `POST /api/ocr/batch/stream`

### Study Notes

- `POST /api/notes/precheck`
- `POST /api/notes/summarize`
- `POST /api/notes/summarize/stream`

## Notes on Security

- Real API keys should never be committed.
- `.env` and `.env.*` are ignored by Git.
- Only `.env.example` should be tracked.

## Current Limitations

- Local OCR still requires a working Python runtime and PaddleOCR installation.
- Handwritten OCR quality depends heavily on image quality and handwriting readability.
- The Markdown preview is intentionally lightweight rather than a full Markdown engine.
- The study-note stage is constrained to the provided Markdown content and does not aim to add outside knowledge.

## Why This Project Is Interesting

- Combines local OCR, optional online OCR, and LLM post-processing in one workflow
- Uses Go concurrency for batch OCR processing
- Uses SSE to stream both OCR progress and LLM output
- Preserves editable per-image OCR output before generating a merged study note
- Designed as an offline-first tool rather than a cloud-only demo

## Roadmap

- Better Markdown rendering support
- Richer logging and performance breakdown
- Native desktop packaging
- ZIP export for batch Markdown results
- More robust note-quality heuristics before LLM calls
