package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Handler func(ctx context.Context, params json.RawMessage) (any, error)

type Server struct {
	log *slog.Logger

	mu      sync.Mutex
	methods map[string]Handler
	subs    map[*subscriber]struct{}
	conns   map[peer]struct{}
}

func NewServer(log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		log:     log,
		methods: map[string]Handler{},
		subs:    map[*subscriber]struct{}{},
		conns:   map[peer]struct{}{},
	}
	s.methods["events.subscribe"] = s.handleSubscribe
	return s
}

func (s *Server) Register(method string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.methods[method] = h
}

// Listen claims the socket path: an exclusive advisory lock on path+".lock"
// makes the claim atomic, so only one daemon may remove a stale socket file and
// bind. The lock is held for the daemon's lifetime and the kernel drops it if
// the process dies, so a crashed daemon never wedges the next start. A second
// daemon fails here, before it can publish an endpoint or serve a request.
func (s *Server) Listen(path string) (net.Listener, error) {
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o700)
	}
	lock, err := acquireSocketLock(path)
	if err != nil {
		return nil, err
	}
	if err := removeStaleSocket(path); err != nil {
		releaseSocketLock(lock)
		return nil, err
	}
	old := syscall.Umask(0o077)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		releaseSocketLock(lock)
		return nil, fmt.Errorf("hikari: cannot listen on %s: %w", path, err)
	}
	return &lockedListener{Listener: ln, lock: lock}, nil
}

// acquireSocketLock takes a non-blocking exclusive flock on path+".lock". The
// leftover .lock file is harmless: the flock, not the file, is the claim.
func acquireSocketLock(path string) (*os.File, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("hikari: cannot open the socket lock %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("hikari: %s is already served (is hikarid already running?)", path)
		}
		return nil, fmt.Errorf("hikari: cannot lock %s: %w", lockPath, err)
	}
	return f, nil
}

func releaseSocketLock(f *os.File) {
	if f == nil {
		return
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// lockedListener keeps the socket and its advisory lock together, so closing
// the listener that Serve owns releases the claim. The lock file itself is
// never removed: a leftover file is fine, only the held flock matters.
type lockedListener struct {
	net.Listener
	mu   sync.Mutex
	lock *os.File
}

func (l *lockedListener) Close() error {
	err := l.Listener.Close()
	l.mu.Lock()
	lock := l.lock
	l.lock = nil
	l.mu.Unlock()
	releaseSocketLock(lock)
	return err
}

// Serve accepts connections on an already claimed listener until ctx is done.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.log.Info("hikarid listening", "socket", ln.Addr().String())

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.closeAll()
			// The listener owns the socket file: close it instead of removing
			// by path, which could unlink a newer daemon's socket.
			_ = ln.Close()
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("hikari: accept: %w", err)
		}
		s.addConn(conn)
		go func() {
			s.serveConn(ctx, conn)
			s.removeConn(conn)
		}()
	}
}

func (s *Server) ListenAndServe(ctx context.Context, path string) error {
	ln, err := s.Listen(path)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

func removeStaleSocket(path string) error {
	conn, err := net.DialTimeout("unix", path, 300*time.Millisecond)
	if err == nil {
		conn.Close()
		return fmt.Errorf("hikari: %s is already served (is hikarid already running?)", path)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return os.Remove(path)
}

func (s *Server) addConn(c peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[c] = struct{}{}
}

func (s *Server) removeConn(c peer) {
	s.mu.Lock()
	var subs []*subscriber
	for sc := range s.subs {
		if sc.conn == c {
			subs = append(subs, sc)
		}
	}
	for _, sc := range subs {
		delete(s.subs, sc)
	}
	delete(s.conns, c)
	s.mu.Unlock()
	for _, sc := range subs {
		sc.stop()
	}
	_ = c.Close()
}

func (s *Server) closeAll() {
	s.mu.Lock()
	conns := make([]peer, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

const maxRequestLine = 1 << 20

func readLine(r *bufio.Reader, max int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := r.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > max {
			return nil, errLineTooLong
		}
		if err == nil {
			return line, nil
		}
		if err != bufio.ErrBufferFull {
			return line, err
		}
	}
}

var errLineTooLong = errors.New("request line too long")

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	// A handler panic must not take the daemon down (the websocket path is
	// protected by net/http's per-connection recover). Log it, drop this
	// connection, and let Serve keep accepting.
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("handler panicked", "err", r)
			_ = conn.Close()
		}
	}()
	r := bufio.NewReader(conn)
	var wmu sync.Mutex
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Hour))
		line, err := readLine(r, maxRequestLine)
		if errors.Is(err, errLineTooLong) {
			s.reply(&wmu, conn, &Response{JSONRPC: "2.0", Error: &Error{Code: CodeInvalidRequest, Message: "request line too long"}})
			return
		}
		if err != nil {
			return
		}
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil || req.JSONRPC != "2.0" || req.Method == "" {
			s.reply(&wmu, conn, &Response{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: CodeInvalidRequest, Message: "malformed request line"}})
			continue
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		resp := s.dispatch(ctx, conn, req)
		s.reply(&wmu, conn, resp)
	}
}

func (s *Server) reply(wmu *sync.Mutex, conn peer, resp *Response) {
	b, err := json.Marshal(resp)
	if err != nil {
		b = []byte(`{"jsonrpc":"2.0","error":{"code":-32003,"message":"reply marshal failed"}}`)
	}
	wmu.Lock()
	defer wmu.Unlock()
	if nc, ok := conn.(net.Conn); ok {
		_ = nc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		defer nc.SetWriteDeadline(time.Time{})
	}
	_, _ = conn.Write(append(b, '\n'))
}

func (s *Server) dispatch(ctx context.Context, conn peer, req Request) *Response {
	s.mu.Lock()
	h := s.methods[req.Method]
	s.mu.Unlock()
	if h == nil {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: CodeMethodNotFound, Message: "unknown method: " + req.Method}}
	}
	cctx := context.WithValue(ctx, connKey{}, conn)
	result, err := h(cctx, req.Params)
	resp := &Response{JSONRPC: "2.0", ID: req.ID}
	if err != nil {
		var re *Error
		if errors.As(err, &re) {
			resp.Error = re
		} else {
			s.log.Error("handler failed", "method", req.Method, "err", err)
			resp.Error = &Error{Code: CodeInternal, Message: err.Error()}
		}
	} else {
		resp.Result = result
	}
	return resp
}

type connKey struct{}

func (s *Server) Publish(event string, params any) {
	b, err := json.Marshal(&Response{JSONRPC: "2.0", Method: event, Params: mustMarshal(params)})
	if err != nil {
		s.log.Error("event marshal failed", "event", event, "err", err)
		return
	}
	b = append(b, '\n')

	s.mu.Lock()
	targets := make([]*subscriber, 0, len(s.subs))
	for sub := range s.subs {
		if sub.wants(event) {
			targets = append(targets, sub)
		}
	}
	s.mu.Unlock()
	for _, sub := range targets {
		sub.deliver(b)
	}
}

func mustMarshal(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
