package browserlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

const EpicLoginURL = "https://www.epicgames.com/id/login?responseType=code"

const EpicUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) EpicGamesLauncher"

var candidates = []string{
	"chromium", "chromium-browser", "google-chrome", "google-chrome-stable",
	"brave", "brave-browser", "microsoft-edge", "vivaldi",
}

func Available() (string, bool) {
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	return "", false
}

func LoginURL(store, fallback string) string {
	if store == "epic" {
		return EpicLoginURL
	}
	return fallback
}

func CodeFromURL(store, raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return ""
	}
	q := u.Query()
	switch store {
	case "epic":
		if code := q.Get("code"); code != "" {
			return code
		}
		return q.Get("authorizationCode")
	case "gog":
		return q.Get("code")
	case "amazon":
		return q.Get("openid.oa2.authorization_code")
	}
	return ""
}

type Session struct {
	cmd     *exec.Cmd
	port    int
	profile string
	exited  chan struct{}
	closed  bool
}

func Open(ctx context.Context, store, loginURL string) (*Session, error) {
	bin, ok := Available()
	if !ok {
		return nil, fmt.Errorf("no Chromium-family browser found; install chromium or use manual login")
	}
	profile, err := os.MkdirTemp("", "hikari-login-")
	if err != nil {
		return nil, fmt.Errorf("cannot create a private browser profile: %w", err)
	}
	port, err := freePort()
	if err != nil {
		os.RemoveAll(profile)
		return nil, fmt.Errorf("cannot reserve a browser debugging port: %w", err)
	}

	args := []string{
		"--user-data-dir=" + profile,
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-session-crashed-bubble",
		"--disable-sync",
		"--password-store=basic",
		"--window-size=1280,900",
	}
	if store == "epic" {
		args = append(args, "--user-agent="+EpicUserAgent)
	}
	args = append(args, loginURL)

	cmd := exec.Command(bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profile)
		return nil, fmt.Errorf("cannot start %s: %w", bin, err)
	}
	s := &Session{cmd: cmd, port: port, profile: profile, exited: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(s.exited)
	}()

	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := s.waitDevTools(readyCtx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Session) WaitCode(ctx context.Context, store string) (string, error) {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	for {
		if code := s.poll(store); code != "" {
			return code, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("login window timed out")
		case <-s.exited:
			return "", fmt.Errorf("login window closed before the store returned a code")
		case <-ticker.C:
		}
	}
}

func (s *Session) Close() {
	if s == nil || s.closed {
		return
	}
	s.closed = true
	if s.cmd.Process != nil {
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGTERM)
	}
	select {
	case <-s.exited:
	case <-time.After(3 * time.Second):
		if s.cmd.Process != nil {
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		}
		<-s.exited
	}
	_ = os.RemoveAll(s.profile)
}

func (s *Session) waitDevTools(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.reachable() {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the browser did not open a debugging endpoint")
		case <-s.exited:
			return fmt.Errorf("the browser exited before it could open the login page")
		case <-ticker.C:
		}
	}
}

func (s *Session) reachable() bool {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", s.port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Session) poll(store string) string {
	client := &http.Client{Timeout: 700 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", s.port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var targets []struct {
		URL  string `json:"url"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return ""
	}
	for _, target := range targets {
		if target.Type != "" && target.Type != "page" {
			continue
		}
		if code := CodeFromURL(store, target.URL); code != "" {
			return code
		}
	}
	return ""
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
