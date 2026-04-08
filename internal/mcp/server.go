package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/yasserrmd/barq-witness/internal/analyzer"
	"github.com/yasserrmd/barq-witness/internal/store"
)

// Server is a stdio-based MCP server. It reads newline-delimited JSON-RPC 2.0
// requests from in and writes responses to out.
type Server struct {
	store    *store.Store
	repoPath string
	in       io.Reader
	out      io.Writer
}

// New returns a new Server backed by st and the git repository at repoPath.
// By default it reads from os.Stdin and writes to os.Stdout.
func New(st *store.Store, repoPath string) *Server {
	return &Server{
		store:    st,
		repoPath: repoPath,
		in:       os.Stdin,
		out:      os.Stdout,
	}
}

// newWithIO returns a Server that reads from in and writes to out.
// Used in tests.
func newWithIO(st *store.Store, repoPath string, in io.Reader, out io.Writer) *Server {
	return &Server{
		store:    st,
		repoPath: repoPath,
		in:       in,
		out:      out,
	}
}

// Run reads JSON-RPC 2.0 requests line by line until ctx is cancelled or
// the input stream is closed (EOF).
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			// EOF -- normal shutdown
			return nil
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, errParse, "parse error: "+err.Error())
			continue
		}

		s.dispatch(&req)
	}
}

// dispatch routes a request to the appropriate handler.
func (s *Server) dispatch(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	default:
		s.writeError(req.ID, errNotFound, "method not found: "+req.Method)
	}
}

// handleInitialize responds to the MCP handshake.
func (s *Server) handleInitialize(req *Request) {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "barq-witness", "version": "1.0.0"},
	}
	s.writeResult(req.ID, result)
}

// handleToolsList returns the list of tools this server exposes.
func (s *Server) handleToolsList(req *Request) {
	tools := []toolDef{
		{
			Name:        "barq_get_report",
			Description: "Run the barq-witness analyzer and return the attention map as JSON.",
			InputSchema: toolInputSchema{
				Type: "object",
				Properties: map[string]schemaProp{
					"from_sha": {Type: "string", Description: "Starting commit SHA (optional)"},
					"to_sha":   {Type: "string", Description: "Ending commit SHA (default: HEAD)"},
					"top_n":    {Type: "integer", Description: "Maximum number of segments to return (default: 10)"},
				},
			},
		},
		{
			Name:        "barq_get_segment",
			Description: "Return details for a specific segment identified by its edit_id.",
			InputSchema: toolInputSchema{
				Type:     "object",
				Required: []string{"edit_id"},
				Properties: map[string]schemaProp{
					"edit_id": {Type: "integer", Description: "The edit ID of the segment to retrieve"},
				},
			},
		},
		{
			Name:        "barq_list_sessions",
			Description: "List recent barq-witness sessions.",
			InputSchema: toolInputSchema{
				Type: "object",
				Properties: map[string]schemaProp{
					"limit": {Type: "integer", Description: "Maximum number of sessions to return (default: 10)"},
				},
			},
		},
		{
			Name:        "barq_get_stats",
			Description: "Return summary statistics from the trace store.",
			InputSchema: toolInputSchema{
				Type:       "object",
				Properties: map[string]schemaProp{},
			},
		},
	}
	s.writeResult(req.ID, toolsListResult{Tools: tools})
}

// handleToolsCall dispatches to the named tool.
func (s *Server) handleToolsCall(req *Request) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(req.ID, errInvalid, "invalid params: "+err.Error())
		return
	}

	switch params.Name {
	case "barq_get_report":
		s.toolGetReport(req.ID, params.Arguments)
	case "barq_get_segment":
		s.toolGetSegment(req.ID, params.Arguments)
	case "barq_list_sessions":
		s.toolListSessions(req.ID, params.Arguments)
	case "barq_get_stats":
		s.toolGetStats(req.ID, params.Arguments)
	default:
		s.writeError(req.ID, errNotFound, "unknown tool: "+params.Name)
	}
}

// toolGetReport runs the analyzer and returns the report.
func (s *Server) toolGetReport(id any, raw json.RawMessage) {
	var args struct {
		FromSHA string `json:"from_sha"`
		ToSHA   string `json:"to_sha"`
		TopN    int    `json:"top_n"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			s.writeError(id, errInvalid, "invalid arguments: "+err.Error())
			return
		}
	}

	if args.ToSHA == "" {
		args.ToSHA = "HEAD"
	}
	if args.TopN <= 0 {
		args.TopN = 10
	}

	report, err := analyzer.Analyze(s.store, s.repoPath, args.FromSHA, args.ToSHA)
	if err != nil {
		s.writeError(id, errInternal, "analyze failed: "+err.Error())
		return
	}

	// Truncate to top_n.
	if args.TopN > 0 && len(report.Segments) > args.TopN {
		report.Segments = report.Segments[:args.TopN]
	}

	s.writeToolJSON(id, report)
}

// toolGetSegment returns a single segment by edit_id.
func (s *Server) toolGetSegment(id any, raw json.RawMessage) {
	var args struct {
		EditID int64 `json:"edit_id"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			s.writeError(id, errInvalid, "invalid arguments: "+err.Error())
			return
		}
	}

	edit, err := s.store.EditByID(args.EditID)
	if err != nil {
		s.writeError(id, errInternal, "store error: "+err.Error())
		return
	}
	if edit == nil {
		s.writeError(id, errNotFound, fmt.Sprintf("edit_id %d not found", args.EditID))
		return
	}

	// Build a minimal segment from the raw edit.
	seg := analyzer.Segment{
		FilePath:    edit.FilePath,
		EditID:      edit.ID,
		SessionID:   edit.SessionID,
		PromptID:    edit.PromptID,
		GeneratedAt: edit.Timestamp,
	}
	if edit.LineStart != nil {
		seg.LineStart = *edit.LineStart
	}
	if edit.LineEnd != nil {
		seg.LineEnd = *edit.LineEnd
	}

	s.writeToolJSON(id, seg)
}

// toolListSessions returns recent sessions.
func (s *Server) toolListSessions(id any, raw json.RawMessage) {
	var args struct {
		Limit int `json:"limit"`
	}
	args.Limit = 10 // default
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			s.writeError(id, errInvalid, "invalid arguments: "+err.Error())
			return
		}
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}

	sessions, err := s.store.RecentSessions(args.Limit)
	if err != nil {
		s.writeError(id, errInternal, "store error: "+err.Error())
		return
	}

	type sessionSummary struct {
		ID        string `json:"id"`
		StartedAt int64  `json:"started_at"`
		Source    string `json:"source"`
		CWD       string `json:"cwd"`
	}

	result := make([]sessionSummary, 0, len(sessions))
	for _, sess := range sessions {
		src := sess.Source
		if src == "" {
			src = "claude-code"
		}
		result = append(result, sessionSummary{
			ID:        sess.ID,
			StartedAt: sess.StartedAt,
			Source:    src,
			CWD:       sess.CWD,
		})
	}

	s.writeToolJSON(id, map[string]any{"sessions": result})
}

// toolGetStats returns aggregate store counts along with tier breakdowns.
func (s *Server) toolGetStats(id any, _ json.RawMessage) {
	stats, err := s.store.GetStats()
	if err != nil {
		s.writeError(id, errInternal, "store error: "+err.Error())
		return
	}

	s.writeToolJSON(id, map[string]any{
		"total_edits":    stats.TotalEdits,
		"total_sessions": stats.TotalSessions,
		"tier1_count":    0,
		"tier2_count":    0,
		"tier3_count":    0,
	})
}

// --- output helpers ----------------------------------------------------------

func (s *Server) writeResult(id any, result any) {
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) writeError(id any, code int, msg string) {
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	})
}

// writeToolJSON marshals v to JSON and returns it as a text tool result.
func (s *Server) writeToolJSON(id any, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		s.writeError(id, errInternal, "marshal error: "+err.Error())
		return
	}
	s.writeResult(id, toolCallResult{
		Content: []toolContent{{Type: "text", Text: string(b)}},
	})
}

func (s *Server) writeResponse(resp Response) {
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	b = append(b, '\n')
	// Ignore write errors -- if the client disconnected, we will hit EOF on the
	// next read and exit cleanly.
	_, _ = s.out.Write(b)
}
