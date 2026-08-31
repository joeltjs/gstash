package web

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/joeltjs/gstash/internal/git"
)

//go:embed static
var staticFS embed.FS

const PortEnvVar = "GSTASH_DASHBOARD_PORT"

var stashRefRe = regexp.MustCompile(`^stash@\{[0-9]{1,4}\}$`)

func LoadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.Trim(strings.TrimSpace(kv[1]), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}

func ResolvePort(dir string) (int, error) {
	for _, p := range []string{filepath.Join(dir, ".env"), ".env"} {
		LoadEnvFile(p)
	}
	raw := strings.TrimSpace(os.Getenv(PortEnvVar))
	if raw == "" {
		return 0, fmt.Errorf("%s is not set - put it in %s/.env or export it. Example: %s=3001", PortEnvVar, dir, PortEnvVar)
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid %s %q - must be a port number between 1 and 65535", PortEnvVar, raw)
	}
	return port, nil
}

type Server struct {
	dir  string
	port int
}

func Serve(dir string, port int) (string, error) {
	s := &Server{dir: dir, port: port}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stashes", s.guard(s.handleStashes))
	mux.HandleFunc("GET /api/diff", s.guard(s.handleDiff))
	mux.HandleFunc("POST /api/apply", s.guard(s.handleApply))
	mux.HandleFunc("POST /api/pop", s.guard(s.handlePop))
	mux.HandleFunc("POST /api/drop", s.guard(s.handleDrop))
	mux.HandleFunc("POST /api/branch", s.guard(s.handleBranch))

	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return "", err
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	handler := securityHeaders(hostAllowlist(csrfGuard(mux, port), port))

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}
	go func() {
		if err := http.Serve(ln, handler); err != nil && err != http.ErrServerClosed {
			fmt.Printf("gstash view server error: %v\n", err)
		}
	}()
	addr := ln.Addr().String()
	openBrowser("http://" + addr)
	return addr, nil
}

// hostAllowlist blocks DNS-rebinding requests: browsers attach the target
// Host header even to cross-site form/fetch posts, so anything that is not
// this loopback server's own host:port never reaches a handler.
func hostAllowlist(next http.Handler, port int) http.Handler {
	ok := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("[::1]:%d", port):     true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ok[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

// csrfGuard blocks cross-site state-changing requests with two layers.
// Layer 1: browsers always attach Origin to cross-origin POSTs; a foreign
// Origin is rejected. Layer 2: mutations must carry the custom header
// X-Requested-With, which cross-site forms and no-preflight fetches cannot
// set. The empty-Origin bypass reported by Strix dies on layer 2.
func csrfGuard(next http.Handler, port int) http.Handler {
	ok := map[string]bool{
		fmt.Sprintf("http://127.0.0.1:%d", port): true,
		fmt.Sprintf("http://localhost:%d", port): true,
		fmt.Sprintf("http://[::1]:%d", port):     true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && !ok[o] {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
			if r.Header.Get("X-Requested-With") != "gstash" {
				http.Error(w, "missing X-Requested-With header", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// guard enforces loopback origin and validates the shared ref parameter.
func (s *Server) guard(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("ref")
		if ref != "" && !stashRefRe.MatchString(ref) {
			httpError(w, fmt.Errorf("invalid ref"))
			return
		}
		next(w, r)
	}
}

func (s *Server) handleStashes(w http.ResponseWriter, r *http.Request) {
	entries, err := git.StashList(s.dir)
	if err != nil {
		httpError(w, err)
		return
	}
	cur, _ := git.CurrentBranch(s.dir)
	writeJSON(w, map[string]any{"stashes": entries, "branch": cur})
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		httpError(w, fmt.Errorf("ref required"))
		return
	}
	diff, err := git.Show(s.dir, ref)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, map[string]any{"diff": diff})
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	out, err := git.Apply(s.dir, r.URL.Query().Get("ref"))
	if err != nil {
		httpError(w, fmt.Errorf("%s: %s", err.Error(), out))
		return
	}
	writeJSON(w, map[string]any{"ok": true, "output": out})
}

func (s *Server) handlePop(w http.ResponseWriter, r *http.Request) {
	out, err := git.Pop(s.dir, r.URL.Query().Get("ref"))
	if err != nil {
		httpError(w, fmt.Errorf("%s: %s", err.Error(), out))
		return
	}
	writeJSON(w, map[string]any{"ok": true, "output": out})
}

func (s *Server) handleDrop(w http.ResponseWriter, r *http.Request) {
	out, err := git.Drop(s.dir, r.URL.Query().Get("ref"))
	if err != nil {
		httpError(w, fmt.Errorf("%s: %s", err.Error(), out))
		return
	}
	writeJSON(w, map[string]any{"ok": true, "output": out})
}

func (s *Server) handleBranch(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		httpError(w, fmt.Errorf("name required"))
		return
	}
	if err := validateBranchName(s.dir, name); err != nil {
		httpError(w, err)
		return
	}
	out, err := git.BranchFromStash(s.dir, r.URL.Query().Get("ref"), name)
	if err != nil {
		httpError(w, fmt.Errorf("%s: %s", err.Error(), out))
		return
	}
	writeJSON(w, map[string]any{"ok": true, "output": out})
}

// validateBranchName delegates to git's own reference validator so no
// hand-rolled allowlist can drift from what git accepts.
func validateBranchName(dir, name string) error {
	if len(name) > 200 {
		return fmt.Errorf("branch name too long")
	}
	out, err := git.Run(dir, "check-ref-format", "--branch", name)
	if err != nil {
		return fmt.Errorf("invalid branch name: %s", out)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	writeJSON(w, map[string]any{"error": err.Error()})
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	} else {
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
