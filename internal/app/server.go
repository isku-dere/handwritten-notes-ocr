package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"handwritten-notes-ocr/internal/llm"
	"handwritten-notes-ocr/internal/markdown"
	"handwritten-notes-ocr/internal/notes"
	"handwritten-notes-ocr/internal/ocr"
)

type ocrResponse struct {
	FileName   string     `json:"fileName"`
	Markdown   string     `json:"markdown"`
	Lines      []ocr.Line `json:"lines"`
	RawText    string     `json:"rawText"`
	DurationMs float64    `json:"durationMs,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type batchOCRStreamEvent struct {
	Index  int         `json:"index"`
	Result ocrResponse `json:"result"`
}

type server struct {
	cfg       Config
	ocrClient *ocr.Client
	noteSvc   *notes.Service
	mux       *http.ServeMux
}

func NewServer(cfg Config) (*http.Server, net.Listener, error) {
	scriptPath, err := prepareOCRScript(cfg)
	if err != nil {
		return nil, nil, err
	}

	ocrClient := &ocr.Client{
		PythonBin:    cfg.PythonBin,
		ScriptPath:   filepath.Clean(scriptPath),
		Language:     cfg.OCRLanguage,
		Timeout:      2 * time.Minute,
		OnlineAPIURL: cfg.OnlineAPIURL,
		APIToken:     cfg.OnlineAPIToken,
	}

	noteSvc := &notes.Service{
		LLM: &llm.Client{
			APIKey:  cfg.QwenAPIKey,
			BaseURL: cfg.QwenBaseURL,
			Model:   cfg.QwenModel,
			Timeout: 90 * time.Second,
		},
	}

	s := &server{
		cfg:       cfg,
		ocrClient: ocrClient,
		noteSvc:   noteSvc,
		mux:       http.NewServeMux(),
	}

	webFS, err := embeddedWebFS()
	if err != nil {
		return nil, nil, err
	}

	s.routes(webFS)

	listener, err := localhostListener(cfg.Port)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on localhost: %w", err)
	}

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           s.loggingMiddleware(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
	}, listener, nil
}

func (s *server) routes(webFS fs.FS) {
	fileServer := http.FileServer(http.FS(webFS))

	s.mux.Handle("/", fileServer)
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/ocr", s.handleOCR)
	s.mux.HandleFunc("/api/ocr/batch", s.handleBatchOCR)
	s.mux.HandleFunc("/api/ocr/batch/stream", s.handleBatchOCRStream)
	s.mux.HandleFunc("/api/notes/precheck", s.handleNotesPrecheck)
	s.mux.HandleFunc("/api/notes/summarize", s.handleSummarizeNotes)
	s.mux.HandleFunc("/api/notes/summarize/stream", s.handleSummarizeNotesStream)
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *server) handleOCR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadMB*1024*1024)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadMB * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "upload too large or malformed form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing image file")
		return
	}
	defer file.Close()

	payload, status, err := s.processUploadedFile(r.Context(), file, header)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, payload)
}

func (s *server) handleBatchOCR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadMB*1024*1024*20)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadMB * 1024 * 1024 * 20); err != nil {
		writeError(w, http.StatusBadRequest, "upload too large or malformed form")
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		if fallback := r.MultipartForm.File["image"]; len(fallback) > 0 {
			files = fallback
		}
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "missing image files")
		return
	}

	results := make([]ocrResponse, len(files))
	limit := s.cfg.OCRConcurrency
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for index, header := range files {
		wg.Add(1)
		go func(i int, h *multipart.FileHeader) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			file, err := h.Open()
			if err != nil {
				results[i] = ocrResponse{
					FileName: h.Filename,
					Error:    "failed to open upload",
				}
				return
			}
			defer file.Close()

			startedAt := time.Now()
			payload, _, processErr := s.processUploadedFile(r.Context(), file, h)
			if processErr != nil {
				results[i] = ocrResponse{
					FileName:   h.Filename,
					Error:      processErr.Error(),
					DurationMs: float64(time.Since(startedAt).Milliseconds()),
				}
				return
			}

			payload.DurationMs = float64(time.Since(startedAt).Milliseconds())
			results[i] = payload
		}(index, header)
	}

	wg.Wait()

	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
	})
}

func (s *server) handleBatchOCRStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadMB*1024*1024*20)
	if err := r.ParseMultipartForm(s.cfg.MaxUploadMB * 1024 * 1024 * 20); err != nil {
		writeError(w, http.StatusBadRequest, "upload too large or malformed form")
		return
	}

	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		if fallback := r.MultipartForm.File["image"]; len(fallback) > 0 {
			files = fallback
		}
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "missing image files")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writeSSE(w, "start", map[string]any{"total": len(files)})
	flusher.Flush()

	limit := s.cfg.OCRConcurrency
	if limit < 1 {
		limit = 1
	}

	type streamResult struct {
		index  int
		result ocrResponse
	}

	sem := make(chan struct{}, limit)
	resultsCh := make(chan streamResult, len(files))
	var wg sync.WaitGroup

	for index, header := range files {
		wg.Add(1)
		go func(i int, h *multipart.FileHeader) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			file, err := h.Open()
			if err != nil {
				resultsCh <- streamResult{
					index: i,
					result: ocrResponse{
						FileName: h.Filename,
						Error:    "failed to open upload",
					},
				}
				return
			}
			defer file.Close()

			startedAt := time.Now()
			payload, _, processErr := s.processUploadedFile(r.Context(), file, h)
			if processErr != nil {
				resultsCh <- streamResult{
					index: i,
					result: ocrResponse{
						FileName:   h.Filename,
						Error:      processErr.Error(),
						DurationMs: float64(time.Since(startedAt).Milliseconds()),
					},
				}
				return
			}

			payload.DurationMs = float64(time.Since(startedAt).Milliseconds())
			resultsCh <- streamResult{index: i, result: payload}
		}(index, header)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	for item := range resultsCh {
		writeSSE(w, "item", batchOCRStreamEvent{
			Index:  item.index,
			Result: item.result,
		})
		flusher.Flush()
	}

	writeSSE(w, "done", map[string]string{"status": "completed"})
	flusher.Flush()
}

func (s *server) handleSummarizeNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Notes []notes.InputNote `json:"notes"`
		Force bool              `json:"force,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !req.Force {
		precheck := s.noteSvc.Precheck(req.Notes)
		if precheck.ShouldConfirm {
			writeJSON(w, http.StatusConflict, precheck)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	result, err := s.noteSvc.GenerateStudyNote(ctx, req.Notes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *server) handleSummarizeNotesStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Notes []notes.InputNote `json:"notes"`
		Force bool              `json:"force,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if !req.Force {
		precheck := s.noteSvc.Precheck(req.Notes)
		if precheck.ShouldConfirm {
			writeSSE(w, "confirm_required", precheck)
			flusher.Flush()
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	writeSSE(w, "start", map[string]string{"status": "streaming"})
	flusher.Flush()

	result, err := s.noteSvc.GenerateStudyNoteStream(ctx, req.Notes, func(delta string) error {
		writeSSE(w, "delta", map[string]string{"text": delta})
		flusher.Flush()
		return nil
	})
	if err != nil {
		writeSSE(w, "error", map[string]string{"error": err.Error()})
		flusher.Flush()
		return
	}

	writeSSE(w, "final", result)
	flusher.Flush()
}

func (s *server) handleNotesPrecheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Notes []notes.InputNote `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	writeJSON(w, http.StatusOK, s.noteSvc.Precheck(req.Notes))
}

func (s *server) processUploadedFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (ocrResponse, int, error) {
	tempFile, err := persistUpload(file, header)
	if err != nil {
		return ocrResponse{}, http.StatusInternalServerError, fmt.Errorf("failed to store upload")
	}
	defer os.Remove(tempFile)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result, err := s.ocrClient.Run(ctx, tempFile)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		return ocrResponse{}, status, err
	}

	markdownText := result.Markdown
	if markdownText == "" {
		markdownText = markdown.Build(result.Lines)
	}

	rawText := result.RawText
	if rawText == "" {
		rawText = strings.Join(extractTexts(result.Lines), "\n")
	}

	return ocrResponse{
		FileName: result.FileName,
		Markdown: markdownText,
		Lines:    result.Lines,
		RawText:  rawText,
	}, http.StatusOK, nil
}

func persistUpload(src multipart.File, header *multipart.FileHeader) (string, error) {
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}

	dst, err := os.CreateTemp("", "handwritten-upload-*"+ext)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return dst.Name(), nil
}

func extractTexts(lines []ocr.Line) []string {
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, line.Text)
	}
	return texts
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{
		"error": message,
	})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func (s *server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
