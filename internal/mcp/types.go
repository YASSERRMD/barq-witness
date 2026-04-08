// Package mcp implements a stdio-based Model Context Protocol server for
// barq-witness. It speaks JSON-RPC 2.0 over stdin/stdout so any MCP-compatible
// AI tool can query the trace store directly.
package mcp

import "encoding/json"

// Request is an inbound JSON-RPC 2.0 message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is an outbound JSON-RPC 2.0 message.
type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// RPCError carries the error code and message for a JSON-RPC error response.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	errParse     = -32700
	errInvalid   = -32600
	errNotFound  = -32601
	errInternal  = -32603
)

// toolInputSchema is a minimal JSON Schema object used in tools/list.
type toolInputSchema struct {
	Type       string                     `json:"type"`
	Properties map[string]schemaProp      `json:"properties,omitempty"`
	Required   []string                   `json:"required,omitempty"`
}

type schemaProp struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// toolDef is the descriptor returned in a tools/list response.
type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema toolInputSchema `json:"inputSchema"`
}

// toolsListResult is the payload for a tools/list response.
type toolsListResult struct {
	Tools []toolDef `json:"tools"`
}

// toolCallResult wraps the text content returned for a tool call.
type toolCallResult struct {
	Content []toolContent `json:"content"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
