package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type Client struct {
	conn net.Conn
	r    *bufio.Reader
	wmu  sync.Mutex
	id   int64

	// Events that arrived while Call was waiting for its reply. They are held
	// here so a later AwaitEvent can still see them instead of losing a
	// terminal progress event to the call that started the job.
	emu   sync.Mutex
	queue []bufferedEvent

	OnEvent func(method string, params json.RawMessage)
}

type bufferedEvent struct {
	method string
	params json.RawMessage
}

func Dial(path string) (*Client, error) {
	conn, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("hikari: daemon is not answering at %s: %w", path, err)
	}
	return &Client{conn: conn, r: bufio.NewReader(conn)}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) Subscribe(events ...string) error {
	var out map[string]any
	params := map[string]any{}
	if len(events) > 0 {
		params["topics"] = events
	}
	return c.Call("events.subscribe", params, &out)
}

func (c *Client) Call(method string, params any, out any) error {
	c.wmu.Lock()
	c.id++
	id := c.id
	req, err := EncodeRequest(id, method, params)
	if err != nil {
		c.wmu.Unlock()
		return err
	}
	_, err = c.conn.Write(append(req, '\n'))
	c.wmu.Unlock()
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("%s: %w", method, err)
		}
		var resp struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Result  json.RawMessage `json:"result"`
			Error   *Error          `json:"error"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return fmt.Errorf("%s: unreadable daemon reply", method)
		}
		if resp.Method != "" && len(resp.ID) == 0 {
			c.bufferEvent(resp.Method, resp.Params)
			continue
		}
		var gotID int64
		if err := json.Unmarshal(resp.ID, &gotID); err != nil || gotID != id {
			continue
		}
		if resp.Error != nil {
			return resp.Error
		}
		if out != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, out); err != nil {
				return fmt.Errorf("%s: unreadable result: %w", method, err)
			}
		}
		return nil
	}
}

// bufferEvent holds one event seen while Call was waiting for its reply.
func (c *Client) bufferEvent(method string, params json.RawMessage) {
	c.emu.Lock()
	c.queue = append(c.queue, bufferedEvent{method: method, params: params})
	c.emu.Unlock()
}

// takeBuffered hands OnEvent the events buffered during Call up to and
// including the first one for method. It reports whether method was found, so
// AwaitEvent can return that event without touching the socket.
func (c *Client) takeBuffered(method string) (json.RawMessage, bool) {
	c.emu.Lock()
	end := len(c.queue)
	found := false
	for i, ev := range c.queue {
		if ev.method == method {
			end, found = i+1, true
			break
		}
	}
	events := c.queue[:end]
	c.queue = c.queue[end:]
	c.emu.Unlock()

	var matched json.RawMessage
	for _, ev := range events {
		if c.OnEvent != nil {
			c.OnEvent(ev.method, ev.params)
		}
		if ev.method == method {
			matched = ev.params
		}
	}
	return matched, found
}

func (c *Client) AwaitEvent(method string, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)
	for {
		if params, ok := c.takeBuffered(method); ok {
			return params, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, errors.New("timed out waiting for event " + method)
		}
		_ = c.conn.SetReadDeadline(deadline)
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		if resp.Method == "" {
			continue
		}
		if c.OnEvent != nil {
			c.OnEvent(resp.Method, resp.Params)
		}
		if resp.Method == method {
			return resp.Params, nil
		}
	}
}
