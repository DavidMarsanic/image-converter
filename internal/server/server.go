// Package server exposes the conversion engine over a small JSON HTTP
// API, bound to loopback only, for the embedded browser-based UI. No
// SSE/job machinery — even a large batch of photos converts in well under
// a second per image, so a plain request/response per batch is enough;
// per-file progress would be complexity nothing here needs.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DavidMarsanic/image-converter/web"
)

const idleTimeout = 30 * time.Minute
const maxUploadBytes = 300 << 20 // 300MB — generous for a batch of photos

// image is one uploaded file held in memory for the lifetime of its
// batch — never written to disk until Convert actually produces an
// output file.
type image struct {
	filename string
	data     []byte
}

type batch struct {
	images map[string]*image // image id -> image
}

type Server struct {
	DefaultOutputDir string
	ctx              context.Context

	mu      sync.Mutex
	batches map[string]*batch

	lastActivity atomic.Int64
}

func New(ctx context.Context, defaultOutputDir string) *Server {
	s := &Server{
		ctx:              ctx,
		DefaultOutputDir: defaultOutputDir,
		batches:          map[string]*batch{},
	}
	s.touch()
	return s
}

func (s *Server) Start(port int) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", fmt.Errorf("starting local server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("POST /api/convert", s.handleConvert)
	mux.HandleFunc("POST /api/dialog/output-folder", s.handleChooseOutputFolder)
	mux.HandleFunc("POST /api/reveal", s.handleReveal)
	mux.HandleFunc("POST /api/open", s.handleOpen)
	mux.Handle("GET /", http.FileServer(http.FS(web.Static)))

	httpSrv := &http.Server{Handler: s.trackActivity(mux)}
	go func() {
		_ = httpSrv.Serve(ln)
	}()
	go s.watchIdle()

	return "http://" + ln.Addr().String(), nil
}

func (s *Server) trackActivity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.touch()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) touch() {
	s.lastActivity.Store(time.Now().Unix())
}

func (s *Server) watchIdle() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		idleFor := time.Now().Unix() - s.lastActivity.Load()
		if idleFor > int64(idleTimeout.Seconds()) {
			os.Exit(0)
		}
	}
}
