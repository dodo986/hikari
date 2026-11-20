package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// The window talks to the daemon over the websocket, and net/http cancels the
// request context as soon as that connection ends. The launch used to inherit
// it, so closing the window (or reloading the page) killed the running game a
// moment later. Only game.stop ends a session.
func TestClosingTheWindowDoesNotKillTheGame(t *testing.T) {
	d, _, srv := startDaemonServer(t)

	wsCtx, cancelWS := context.WithCancel(context.Background())
	defer cancelWS()
	if _, err := startWS(wsCtx, discardLogger(), srv, d.art); err != nil {
		t.Fatal(err)
	}
	ep := readEndpointFile(t)

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDial()
	conn, _, err := websocket.Dial(dialCtx, ep.URL+"?token="+ep.Token, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", ep.URL, err)
	}

	call := func(id int, method string, params any) error {
		req, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := conn.Write(ctx, websocket.MessageText, req); err != nil {
			return err
		}
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return err
			}
			var resp struct {
				ID    int `json:"id"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(data, &resp) != nil || resp.ID != id {
				continue
			}
			if resp.Error != nil {
				return errString(resp.Error.Message)
			}
			return nil
		}
	}

	if err := call(1, "game.launch", map[string]any{"store": "epic", "id": "nativegame"}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !d.games.IsRunning("epic/nativegame") {
		time.Sleep(10 * time.Millisecond)
	}
	if !d.games.IsRunning("epic/nativegame") {
		t.Fatal("the game never started")
	}

	// The window goes away. A canceled launch context kills the process group
	// within milliseconds, so a short watch is enough to catch the regression.
	_ = conn.Close(websocket.StatusNormalClosure, "window closed")
	for i := 0; i < 50; i++ {
		if !d.games.IsRunning("epic/nativegame") {
			t.Fatal("closing the window killed the running game")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := d.games.Stop("epic/nativegame"); err != nil {
		t.Logf("stop: %v", err)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
