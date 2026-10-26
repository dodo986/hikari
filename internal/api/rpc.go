package api

import (
	"encoding/json"
	"fmt"
)

const (
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeNotAvailable   = -32000
	CodeUnsupported    = -32001
	CodeBusy           = -32002
	CodeNotFound       = -32004
	CodeInternal       = -32003
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("rpc %d: %s", e.Code, e.Message) }

func Errorf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func EncodeRequest(id any, method string, params any) ([]byte, error) {
	req := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id,omitempty"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	return json.Marshal(req)
}
