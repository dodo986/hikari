package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type wsHarness struct {
	srv   *Server
	url   string
	token string
	stop  func()
}

func startWS(t *testing.T, opts ...func(*WSConfig)) *wsHarness {
	t.Helper()
	srv := NewServer(nil)
	srv.Register("ping", func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"pong": true}, nil
	})
	srv.Register("events.echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		srv.Publish("library.changed", map[string]any{"store": "epic"})
		return map[string]any{"ok": true}, nil
	})
	cfg := WSConfig{Token: "test-token", Origins: []string{"wails://wails", "http://localhost:5173"}}
	for _, o := range opts {
		o(&cfg)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.ServeWS(ctx, ln, cfg)
	}()
	h := &wsHarness{srv: srv, url: "ws://" + ln.Addr().String() + "/rpc", token: cfg.Token}
	h.stop = func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Log("websocket server did not stop in time")
		}
	}
	t.Cleanup(h.stop)
	return h
}

func dialWS(t *testing.T, url string, header http.Header) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: header})
	return conn, err
}

func TestWSAnswersTheSameMethods(t *testing.T) {
	h := startWS(t)
	conn, err := dialWS(t, h.url+"?token="+h.token, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	req, err := EncodeRequest(1, "ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, req); err != nil {
		t.Fatal(err)
	}
	typ, data, err := conn.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("frame type = %v, want text", typ)
	}
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	body, _ := json.Marshal(resp.Result)
	if string(body) != `{"pong":true}` {
		t.Fatalf("result = %s", body)
	}
}

func TestWSDeliversEventsOverTheSameConnection(t *testing.T) {
	h := startWS(t)
	conn, err := dialWS(t, h.url+"?token="+h.token, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	sub, err := EncodeRequest(1, "events.subscribe", map[string]any{"topics": []string{"library.changed"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, sub); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.Read(context.Background()); err != nil {
		t.Fatal(err)
	}

	req, err := EncodeRequest(2, "events.echo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, req); err != nil {
		t.Fatal(err)
	}

	deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, data, err := conn.Read(deadline)
		if err != nil {
			t.Fatalf("event never arrived: %v", err)
		}
		var frame Response
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Fatal(err)
		}
		if frame.Method == "library.changed" {
			return
		}
	}
}

func TestWSRejectsABadToken(t *testing.T) {
	h := startWS(t)
	conn, err := dialWS(t, h.url+"?token=wrong", nil)
	if err == nil {
		conn.CloseNow()
		t.Fatal("a wrong token was accepted")
	}
	if conn, err := dialWS(t, h.url, nil); err == nil {
		conn.CloseNow()
		t.Fatal("a connection without a token was accepted")
	}
}

func TestWSRejectsAForeignOrigin(t *testing.T) {
	h := startWS(t)
	header := http.Header{}
	header.Set("Origin", "https://evil.example")
	if conn, err := dialWS(t, h.url+"?token="+h.token, header); err == nil {
		conn.CloseNow()
		t.Fatal("a foreign origin was accepted")
	}
	header.Set("Origin", "wails://wails")
	conn, err := dialWS(t, h.url+"?token="+h.token, header)
	if err != nil {
		t.Fatalf("the host origin was refused: %v", err)
	}
	conn.CloseNow()
}

func TestWSRejectsAForeignHost(t *testing.T) {
	h := startWS(t)
	addr := tcpAddrOf(t, h.url)
	handler := h.srv.wsHandler(WSConfig{Token: h.token}, addr)

	evil := httptest.NewRequest("GET", "http://attacker.example/rpc?token="+h.token, nil)
	evil.Host = "attacker.example"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, evil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign host got %d, want 403", rec.Code)
	}

	good := httptest.NewRequest("GET", "http://"+addr.String()+"/rpc?token="+h.token, nil)
	good.Host = addr.String()
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, good)
	if rec.Code == http.StatusForbidden {
		t.Fatal("the loopback host was refused")
	}
}

func tcpAddrOf(t *testing.T, url string) *net.TCPAddr {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", strings.TrimPrefix(url, "ws://")[:len(strings.TrimPrefix(url, "ws://"))-len("/rpc")])
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

func TestWSRejectsAnEmptyServerToken(t *testing.T) {
	srv := NewServer(nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	err = srv.ServeWS(context.Background(), ln, WSConfig{})
	if err == nil {
		t.Fatal("a server without a token started anyway")
	}
}

func TestHostAllowed(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 45231}
	cases := map[string]bool{
		"127.0.0.1:45231":  true,
		"localhost:45231":  true,
		"127.0.0.1:1":      false,
		"attacker.example": false,
		"":                 false,
	}
	for host, want := range cases {
		if got := hostAllowed(host, addr); got != want {
			t.Errorf("hostAllowed(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestSameToken(t *testing.T) {
	if !sameToken("abc", "abc") {
		t.Error("equal tokens were refused")
	}
	if sameToken("abc", "abd") || sameToken("", "") || sameToken("abc", "") {
		t.Error("a different or empty token was accepted")
	}
}
