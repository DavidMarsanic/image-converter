package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/DavidMarsanic/image-converter/internal/browser"
	"github.com/DavidMarsanic/image-converter/internal/dialog"
	"github.com/DavidMarsanic/image-converter/internal/engine"
)

type uploadedImage struct {
	ID       string       `json:"id"`
	Filename string       `json:"filename"`
	Info     *engine.Info `json:"info,omitempty"`
	Error    string       `json:"error,omitempty"`
}

// handleUpload accepts one or more images, reports each one's format and
// dimensions, and holds the original bytes in memory (never written to
// disk) under a new batch id so a later /api/convert call can act on the
// same files without a second upload.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid upload", "code": "bad-request"})
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no files uploaded", "code": "bad-request"})
		return
	}

	b := &batch{images: map[string]*image{}}
	var results []uploadedImage

	for _, header := range files {
		f, err := header.Open()
		if err != nil {
			results = append(results, uploadedImage{Filename: header.Filename, Error: "couldn't read upload"})
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			results = append(results, uploadedImage{Filename: header.Filename, Error: "couldn't read upload"})
			continue
		}

		id := newID()
		name := sanitizeFilename(header.Filename)
		b.images[id] = &image{filename: name, data: data}

		info, err := engine.Inspect(data)
		if err != nil {
			results = append(results, uploadedImage{ID: id, Filename: name, Error: err.Error()})
			continue
		}
		results = append(results, uploadedImage{ID: id, Filename: name, Info: info})
	}

	batchID := newID()
	s.mu.Lock()
	s.batches[batchID] = b
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"batchId": batchID, "images": results})
}

type convertRequest struct {
	BatchID      string   `json:"batchId"`
	IDs          []string `json:"ids"`
	Format       string   `json:"format"`
	Quality      int      `json:"quality"`
	MaxDimension int      `json:"maxDimension"`
	OutputDir    string   `json:"outputDir"`
}

type convertResult struct {
	ID       string `json:"id"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleConvert converts the requested images in a batch to a single
// target format and writes each result to the output directory, named
// "<stem>.<ext>" (uniquified on collision). Once every image in the batch
// has been either converted or explicitly errored, the batch's in-memory
// bytes are dropped.
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	var req convertRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	s.mu.Lock()
	b, ok := s.batches[req.BatchID]
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown batch — upload again", "code": "bad-request"})
		return
	}

	outDir := req.OutputDir
	if outDir == "" {
		outDir = s.DefaultOutputDir
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "creating output folder: " + err.Error()})
		return
	}

	opts := engine.Options{
		Format:       engine.Format(req.Format),
		Quality:      req.Quality,
		MaxDimension: req.MaxDimension,
	}

	var results []convertResult
	for _, id := range req.IDs {
		img, ok := b.images[id]
		if !ok {
			results = append(results, convertResult{ID: id, Error: "unknown image in this batch"})
			continue
		}
		out, err := engine.Convert(img.data, opts)
		if err != nil {
			results = append(results, convertResult{ID: id, Error: err.Error()})
			continue
		}
		outPath, err := writeConverted(outDir, img.filename, out.Extension, out.Data)
		if err != nil {
			results = append(results, convertResult{ID: id, Error: err.Error()})
			continue
		}
		results = append(results, convertResult{
			ID: id, Filename: filepath.Base(outPath), Path: outPath,
			Width: out.Width, Height: out.Height, Bytes: len(out.Data),
		})
	}

	s.mu.Lock()
	delete(s.batches, req.BatchID)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"results": results, "outputDir": outDir})
}

func writeConverted(outputDir, originalName, ext string, data []byte) (string, error) {
	stem := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	if stem == "" {
		stem = "image"
	}
	outPath := uniquePath(outputDir, stem, ext)
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return "", fmt.Errorf("saving converted image: %w", err)
	}
	return outPath, nil
}

func uniquePath(dir, stem, ext string) string {
	candidate := filepath.Join(dir, stem+ext)
	for i := 2; fileExists(candidate); i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
	}
	return candidate
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	if name == "." || name == ".." || name == "" {
		return "image"
	}
	return name
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleChooseOutputFolder(w http.ResponseWriter, r *http.Request) {
	path, err := dialog.ChooseFolder()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := browser.Reveal(req.Path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := browser.Open(req.Path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body", "code": "bad-request"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
