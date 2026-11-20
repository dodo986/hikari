package api

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

type subscriber struct {
	conn   peer
	mu     sync.Mutex
	topics map[string]bool
	frames chan []byte
	done   chan struct{}
	once   sync.Once
}

func (s *Server) handleSubscribe(ctx context.Context, params json.RawMessage) (any, error) {
	conn, ok := ctx.Value(connKey{}).(peer)
	if !ok {
		return nil, errors.New("no connection in context")
	}
	var p struct {
		Topics []string `json:"topics"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, Errorf(CodeInvalidParams, "params must be an object with an optional topics array")
		}
	}
	topics := map[string]bool{}
	for _, t := range p.Topics {
		topics[t] = true
	}
	// Subscribing is an upsert per connection: a second events.subscribe on the
	// same connection must not register a duplicate subscriber, which would
	// deliver every event twice and leak a pump goroutine.
	s.mu.Lock()
	var sub *subscriber
	for sc := range s.subs {
		if sc.conn == conn {
			sub = sc
			break
		}
	}
	fresh := sub == nil
	if fresh {
		sub = &subscriber{
			conn:   conn,
			topics: topics,
			frames: make(chan []byte, 128),
			done:   make(chan struct{}),
		}
		s.subs[sub] = struct{}{}
	} else {
		sub.mu.Lock()
		sub.topics = topics
		sub.mu.Unlock()
	}
	s.mu.Unlock()
	if fresh {
		go sub.pump()
	}
	if len(p.Topics) == 0 {
		return map[string]any{"topics": []string{}}, nil
	}
	return map[string]any{"topics": p.Topics}, nil
}

func (sub *subscriber) wants(event string) bool {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if len(sub.topics) == 0 {
		return true
	}
	return sub.topics[event]
}

func (sub *subscriber) deliver(b []byte) {
	select {
	case sub.frames <- b:
	default:
		select {
		case <-sub.frames:
		default:
		}
		select {
		case sub.frames <- b:
		case <-sub.done:
		}
	}
}

func (sub *subscriber) pump() {
	for {
		select {
		case <-sub.done:
			return
		case b := <-sub.frames:
			if _, err := sub.conn.Write(b); err != nil {
				sub.stop()
				return
			}
		}
	}
}

func (sub *subscriber) stop() {
	sub.once.Do(func() { close(sub.done) })
}

func EventTopicFilter(events ...string) map[string]bool {
	m := map[string]bool{}
	for _, e := range events {
		m[e] = true
	}
	return m
}
