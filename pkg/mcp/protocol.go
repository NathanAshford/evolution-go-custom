// Package mcp implements a Model Context Protocol server for Evolution GO,
// letting MCP clients (Claude, ChatGPT and others) send WhatsApp messages and
// search the message archive.
//
// The transport is Streamable HTTP: a single endpoint that accepts JSON-RPC 2.0
// requests over POST. This is the transport both Claude and ChatGPT custom
// connectors speak.
package mcp

import "encoding/json"

// protocolVersion is the MCP revision this server implements. Clients that ask
// for a different revision are answered with this one, per the spec's
// version-negotiation rules.
const protocolVersion = "2024-11-05"

const (
	serverName    = "evolution-go"
	serverVersion = "1.0.0"
)

// JSON-RPC 2.0 error codes. -32000..-32099 is the implementation-defined range.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	codeUnauthorized   = -32001
)

// rpcRequest is one JSON-RPC call. ID is absent for notifications, which must
// not be answered.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (r rpcRequest) isNotification() bool {
	return len(r.ID) == 0 || string(r.ID) == "null"
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func newResult(id json.RawMessage, result any) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func newError(id json.RawMessage, code int, message string) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}

// --- initialize ---

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

type serverCapabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// --- tools ---

// tool is one entry of tools/list. InputSchema must be a JSON Schema object.
type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// contentBlock is a piece of a tool result. Only text blocks are produced here.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// callToolResult carries the tool output. IsError signals a tool-level failure
// (bad number, instance offline) as opposed to a protocol error — the model
// sees the message and can correct itself.
type callToolResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

func textResult(text string) *callToolResult {
	return &callToolResult{Content: []contentBlock{{Type: "text", Text: text}}}
}

func errorResult(text string) *callToolResult {
	return &callToolResult{Content: []contentBlock{{Type: "text", Text: text}}, IsError: true}
}

// jsonResult renders a value as pretty JSON in a text block, which is how MCP
// tools conventionally return structured data.
func jsonResult(v any) *callToolResult {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorResult("failed to encode result: " + err.Error())
	}
	return textResult(string(data))
}
