package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/autokeren/payfox/internal/model"
)

// Server is a stdio MCP server (JSON-RPC over stdin/stdout).
// Spawned from a command (e.g. the Ghostfox stealth browser engine).
type Server struct {
	Name    string
	Command []string
	Env     map[string]string

	proc   *exec.Cmd
	stdin  io.WriteCloser
	lines  chan []byte
	nextID int
	mu     sync.Mutex
}

func NewServer(name string, command []string, env map[string]string) *Server {
	return &Server{Name: name, Command: command, Env: env, nextID: 1}
}

func (s *Server) Start(ctx context.Context) error {
	if len(s.Command) == 0 {
		return fmt.Errorf("mcp %s: empty command", s.Name)
	}
	s.proc = exec.CommandContext(ctx, s.Command[0], s.Command[1:]...)
	s.proc.Env = os.Environ()
	for k, v := range s.Env {
		s.proc.Env = append(s.proc.Env, k+"="+v)
	}
	stdin, err := s.proc.StdinPipe()
	if err != nil {
		return err
	}
	s.stdin = stdin
	stdout, err := s.proc.StdoutPipe()
	if err != nil {
		return err
	}
	s.lines = make(chan []byte, 256)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				s.lines <- []byte(line)
			}
		}
		close(s.lines)
	}()
	if err := s.proc.Start(); err != nil {
		return err
	}
	// initialize handshake
	var resp map[string]any
	if err := s.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "kancil", "version": "0.1.0"},
	}, &resp); err != nil {
		return err
	}
	// initialized notification (no response expected)
	_ = s.notify("notifications/initialized")
	return nil
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *Server) send(m rpcMessage) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.stdin.Write(append(raw, '\n'))
	return err
}

func (s *Server) notify(method string) error {
	return s.send(rpcMessage{JSONRPC: "2.0", Method: method})
}

func (s *Server) call(ctx context.Context, method string, params map[string]any, out any) error {
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.mu.Unlock()
	raw, _ := json.Marshal(params)
	if err := s.send(rpcMessage{JSONRPC: "2.0", ID: id, Method: method, Params: raw}); err != nil {
		return err
	}
	deadline := time.Now().Add(120 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-s.lines:
			if !ok {
				return fmt.Errorf("mcp %s: stream closed", s.Name)
			}
			var msg rpcMessage
			if err := json.Unmarshal(line, &msg); err != nil || msg.ID != id {
				continue // skip notifications / other responses
			}
			if msg.Error != nil {
				return fmt.Errorf("mcp %s: %s", s.Name, msg.Error.Message)
			}
			if out != nil {
				return json.Unmarshal(msg.Result, out)
			}
			return nil
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("mcp %s: timeout waiting for %s", s.Name, method)
		}
	}
}

// ListTools fetches the server's tools and converts them into
// kancil tool definitions (OpenAI function format).
func (s *Server) ListTools(ctx context.Context) ([]model.ToolDefinition, error) {
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := s.call(ctx, "tools/list", map[string]any{}, &result); err != nil {
		return nil, err
	}
	var defs []model.ToolDefinition
	for _, t := range result.Tools {
		defs = append(defs, model.ToolDefinition{
			Type: "function",
			Function: model.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return defs, nil
}

// CallTool invokes a tool and returns the text of its response.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	params := map[string]any{"name": name, "arguments": args}
	if err := s.call(ctx, "tools/call", params, &result); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

func (s *Server) Close() {
	if s.stdin != nil {
		_, _ = s.stdin.Write([]byte("\n")) // nudge
	}
	if s.proc != nil && s.proc.Process != nil {
		_ = s.proc.Process.Kill()
	}
}
