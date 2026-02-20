// Package mcp implements the Model Context Protocol server.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/rsned/spacemolt-crafting-server/internal/crafting/engine"
)

// Server implements an MCP server over stdio.
type Server struct {
	engine   *engine.Engine
	logger   *slog.Logger
	handlers map[string]MethodHandler
}

// MethodHandler handles a specific JSON-RPC method.
type MethodHandler func(ctx context.Context, params json.RawMessage) (any, error)

// NewServer creates a new MCP server.
func NewServer(eng *engine.Engine, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	
	s := &Server{
		engine:   eng,
		logger:   logger,
		handlers: make(map[string]MethodHandler),
	}
	
	// Register handlers
	s.handlers["initialize"] = s.handleInitialize
	s.handlers["tools/list"] = s.handleToolsList
	s.handlers["tools/call"] = s.handleToolsCall
	
	return s
}

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents a JSON-RPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	ErrCodeParse       = -32700
	ErrCodeInvalidReq  = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal    = -32603
)

// Run starts the server, reading from stdin and writing to stdout.
func (s *Server) Run(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout
	
	s.logger.Info("MCP server starting")
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading input: %w", err)
		}
		
		resp := s.handleRequest(ctx, line)
		if resp != nil {
			if err := s.writeResponse(writer, resp); err != nil {
				s.logger.Error("failed to write response", "error", err)
			}
		}
	}
}

// handleRequest processes a single request.
func (s *Server) handleRequest(ctx context.Context, data []byte) *Response {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return &Response{
			JSONRPC: "2.0",
			Error: &Error{
				Code:    ErrCodeParse,
				Message: "Parse error",
				Data:    err.Error(),
			},
		}
	}
	
	s.logger.Debug("received request", "method", req.Method, "id", req.ID)
	
	handler, ok := s.handlers[req.Method]
	if !ok {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    ErrCodeMethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
	
	result, err := handler(ctx, req.Params)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    ErrCodeInternal,
				Message: err.Error(),
			},
		}
	}
	
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// writeResponse writes a JSON-RPC response.
func (s *Server) writeResponse(w io.Writer, resp *Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}
	
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// Initialize result.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (any, error) {
	return InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "spacemolt-crafting",
			Version: "0.1.0",
		},
		Capabilities: Capabilities{
			Tools: &ToolsCapability{},
		},
	}, nil
}

// ToolsListResult is the response for tools/list.
type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

func (s *Server) handleToolsList(ctx context.Context, params json.RawMessage) (any, error) {
	return ToolsListResult{
		Tools: GetToolDefinitions(),
	}, nil
}

// ToolCallParams are the parameters for tools/call.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResult is the response for tools/call.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, error) {
	var p ToolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	
	s.logger.Debug("calling tool", "name", p.Name)
	
	result, err := s.callTool(ctx, p.Name, p.Arguments)
	if err != nil {
		return ToolCallResult{}, fmt.Errorf("tool call failed: %w", err)
	}
	
	// Marshal result to JSON for text output
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling result: %w", err)
	}
	
	return ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: string(resultJSON)}},
	}, nil
}

// callTool dispatches to the appropriate tool handler.
func (s *Server) callTool(ctx context.Context, name string, args json.RawMessage) (any, error) {
	switch name {
	case "craft_query":
		return s.toolCraftQuery(ctx, args)
	case "craft_path_to":
		return s.toolCraftPathTo(ctx, args)
	case "recipe_lookup":
		return s.toolRecipeLookup(ctx, args)
	case "skill_craft_paths":
		return s.toolSkillCraftPaths(ctx, args)
	case "component_uses":
		return s.toolComponentUses(ctx, args)
	case "bill_of_materials":
		return s.toolBillOfMaterials(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
