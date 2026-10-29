package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// WSConfig is what a browser cannot prove by itself: the shared token every
// connection must carry, and the origins allowed to open one.
type WSConfig struct {
	Token   string
	Origins []string
	// Assets serves non RPC paths on the same loopback listener, behind the
	// same token check. It lets a browser reach local artwork.
	Assets http.Handler
}

const (
	wsPath        = "/rpc"
	wsPingEvery   = 30 * time.Second
	wsWriteWindow = 10 * time.Second

	CloseUnauthorized = 4401
)

// ServeWS answers JSON-RPC over websocket on an already bound listener, so the
// caller can read the port and publish it before the first client arrives.
func (s *Server) ServeWS(ctx context.Context, ln net.Listener, cfg WSConfig) error {
	if cfg.Token == "" {
		return errors.New("hikari: the websocket endpoint needs a token")
	}
	srv := &http.Server{
		Handler:           s.wsHandler(cfg, ln.Addr()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	s.log.Info("hikarid websocket listening", "addr", ln.Addr().String())
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) wsHandler(cfg WSConfig, addr net.Addr) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		websocketUpgrade := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
		if r.URL.Path != wsPath && !websocketUpgrade {
			if cfg.Assets == nil {
				http.NotFound(w, r)
				return
			}
			if !hostAllowed(r.Host, addr) || !sameToken(r.URL.Query().Get("token"), cfg.Token) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			cfg.Assets.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != wsPath {
			http.NotFound(w, r)
			return
		}
		if !hostAllowed(r.Host, addr) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		authorised := sameToken(r.URL.Query().Get("token"), cfg.Token)
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns:  cfg.Origins,
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		if !authorised {
			_ = conn.Close(websocket.StatusCode(CloseUnauthorized), "token rejected")
			return
		}
		conn.SetReadLimit(maxRequestLine)
		s.serveWS(r.Context(), conn)
	})
}

// hostAllowed blocks DNS rebinding: the browser sends the hostname it resolved,
// which must be the loopback address we actually listen on.
func hostAllowed(host string, addr net.Addr) bool {
	port := ""
	if a, ok := addr.(*net.TCPAddr); ok {
		port = itoa(a.Port)
	}
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	switch strings.ToLower(name) {
	case "127.0.0.1", "localhost", "::1", "[::1]":
		return port == "" || strings.HasSuffix(host, ":"+port)
	}
	return false
}

func sameToken(got, want string) bool {
	if want == "" {
		return false
	}
	a, b := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

func (s *Server) serveWS(ctx context.Context, conn *websocket.Conn) {
	p := &wsPeer{conn: conn}
	s.addConn(p)
	defer s.removeConn(p)
	defer conn.CloseNow()

	pingCtx, stopPing := context.WithCancel(ctx)
	defer stopPing()
	go p.ping(pingCtx)

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			_ = conn.Close(websocket.StatusUnsupportedData, "text frames only")
			return
		}
		var req Request
		if err := json.Unmarshal(data, &req); err != nil || req.JSONRPC != "2.0" || req.Method == "" {
			_ = p.writeFrame(&Response{JSONRPC: "2.0", ID: req.ID, Error: &Error{Code: CodeInvalidRequest, Message: "malformed request"}})
			continue
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		if err := p.writeFrame(s.dispatch(ctx, p, req)); err != nil {
			return
		}
	}
}

// wsPeer frames what the unix socket would have written as lines.
type wsPeer struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (p *wsPeer) Write(b []byte) (int, error) {
	payload := b
	for len(payload) > 0 && (payload[len(payload)-1] == '\n' || payload[len(payload)-1] == '\r') {
		payload = payload[:len(payload)-1]
	}
	if len(payload) == 0 {
		return len(b), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), wsWriteWindow)
	defer cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.conn.Write(ctx, websocket.MessageText, payload); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (p *wsPeer) Close() error {
	return p.conn.CloseNow()
}

func (p *wsPeer) writeFrame(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = p.Write(b)
	return err
}

func (p *wsPeer) ping(ctx context.Context) {
	tick := time.NewTicker(wsPingEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			pingCtx, cancel := context.WithTimeout(ctx, wsWriteWindow)
			err := p.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				_ = p.conn.CloseNow()
				return
			}
		}
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
