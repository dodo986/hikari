package api

// peer is a client connection. The unix socket and a websocket both satisfy it,
// so the dispatcher, the event bus and every handler stay transport agnostic.
type peer interface {
	Write(p []byte) (int, error)
	Close() error
}
