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

	OnEvent func(method string, params json.RawMessage)
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
			if c.OnEvent != nil {
				c.OnEvent(resp.Method, resp.Params)
			}
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

func (c *Client) AwaitEvent(method string, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)
	for {
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
