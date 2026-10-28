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

func (s *Server) ListenAndServe(ctx context.Context, path string) error {
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o700)
	}
	if err := removeStaleSocket(path); err != nil {
		return err
	}
	old := syscall.Umask(0o077)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		return fmt.Errorf("hikari: cannot listen on %s: %w", path, err)
	}

	s.log.Info("hikarid listening", "socket", path)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.closeAll()
			_ = os.Remove(path)
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
	sub := (*subscriber)(nil)
	for sc := range s.subs {
		if sc.conn == c {
			sub = sc
		}
	}
	if sub != nil {
		delete(s.subs, sub)
	}
	delete(s.conns, c)
	s.mu.Unlock()
	if sub != nil {
		sub.stop()
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
