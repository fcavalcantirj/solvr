package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
)

// A /v1/mcp tool call runs as one or more requests through the API's own router: the same
// route, middleware, credential check, validation and error answer as a REST call. The
// dispatched request carries the MCP call's request id and client address, and is marked
// in-process so API usage counts the MCP call once.

// mcpCall is one tools/call: its context and the MCP request it came in on.
type mcpCall struct {
	h         *MCPHandler
	ctx       context.Context
	from      *http.Request
	requestID string
}

// callerAuth is the Authorization header the caller sent to /v1/mcp ("" = anonymous).
func (c *mcpCall) callerAuth() string {
	return c.from.Header.Get("Authorization")
}

// mcpResult is a tool's answer: its text, and whether it reports a failure.
type mcpResult struct {
	text    string
	isError bool
}

func (res mcpResult) rpc() map[string]interface{} {
	out := map[string]interface{}{
		"content": []map[string]interface{}{{"type": "text", "text": res.text}},
	}
	if res.isError {
		out["isError"] = true
	}
	return out
}

func mcpLines(lines ...string) mcpResult {
	return mcpResult{text: strings.Join(lines, "\n")}
}

// mcpFailure is the result of a failed call: the error, and the request id the API gave it.
func mcpFailure(tool string, err error) map[string]interface{} {
	return mcpResult{text: mcpFailureText(tool, err), isError: true}.rpc()
}

func mcpFailureText(tool string, err error) string {
	text := "Error executing " + tool + ": " + err.Error()
	var apiErr *mcpAPIError
	if errors.As(err, &apiErr) && apiErr.requestID != "" {
		text += "\nrequest id: " + apiErr.requestID
	}
	return text
}

// mcpAPIError is an error the API answered (status 0: the room stream ended with it).
type mcpAPIError struct {
	status                   int
	code, message, requestID string
}

func (e *mcpAPIError) Error() string {
	var parts []string
	for _, p := range []string{e.code, e.message} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	detail := ""
	if len(parts) > 0 {
		detail = ": " + strings.Join(parts, ": ")
	}
	if e.status == 0 {
		return "Stream ended" + detail
	}
	return fmt.Sprintf("API request failed: %d %s%s", e.status, http.StatusText(e.status), detail)
}

func mcpAPIErrorFrom(w *mcpResponseWriter) *mcpAPIError {
	var envelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.body.Bytes(), &envelope)
	requestID := envelope.Error.RequestID
	if requestID == "" {
		requestID = w.header.Get("X-Request-ID")
	}
	return &mcpAPIError{status: w.status, code: envelope.Error.Code, message: envelope.Error.Message, requestID: requestID}
}

// apiRequest is one request a tool sends to the API.
type apiRequest struct {
	method  string
	path    string // escaped: build it with apiPath
	query   url.Values
	headers map[string]string
	auth    string      // the Authorization header; "" sends none
	body    interface{} // nil sends no body
}

// apiPath fills each %s of format with a path-escaped parameter.
func apiPath(format string, params ...string) string {
	escaped := make([]interface{}, len(params))
	for i, p := range params {
		escaped[i] = url.PathEscape(p)
	}
	return fmt.Sprintf(format, escaped...)
}

// mcpResponseWriter collects the API's answer to a dispatched request. stream, when set,
// receives the body of a 200 text/event-stream answer as it is written.
type mcpResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
	stream func([]byte)
}

func newMCPResponseWriter() *mcpResponseWriter {
	return &mcpResponseWriter{header: http.Header{}}
}

func (w *mcpResponseWriter) Header() http.Header { return w.header }

func (w *mcpResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *mcpResponseWriter) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	if w.stream != nil && w.status == http.StatusOK && strings.HasPrefix(w.header.Get("Content-Type"), "text/event-stream") {
		w.stream(p)
		return len(p), nil
	}
	return w.body.Write(p)
}

// Flush keeps streaming handlers working: they assert on http.Flusher.
func (w *mcpResponseWriter) Flush() {}

// serve runs req through the API router with ctx; the answer is in w.
func (c *mcpCall) serve(ctx context.Context, req apiRequest, w *mcpResponseWriter) error {
	if c.h.dispatcher == nil {
		return errors.New("this MCP endpoint is not connected to the API")
	}
	target := req.path
	if q := req.query.Encode(); q != "" {
		target += "?" + q
	}
	var body io.Reader = http.NoBody
	if req.body != nil {
		raw, err := json.Marshal(req.body)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	// A fresh context: no value of the MCP request (its chi route context, a caller identity)
	// reaches the dispatched request, which is still cancelled with the MCP call.
	reqCtx, cancel := context.WithCancel(apimiddleware.WithInProcessCall(context.Background()))
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()
	httpReq, err := http.NewRequestWithContext(reqCtx, req.method, target, body)
	if err != nil {
		return err
	}
	if req.body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.RequestURI = httpReq.URL.RequestURI()
	httpReq.RemoteAddr = c.from.RemoteAddr
	httpReq.Host = c.from.Host
	if ua := c.from.Header.Get("User-Agent"); ua != "" {
		httpReq.Header.Set("User-Agent", ua)
	}
	if c.requestID != "" {
		httpReq.Header.Set("X-Request-ID", c.requestID)
	}
	if req.auth != "" {
		httpReq.Header.Set("Authorization", req.auth)
	}
	for name, value := range req.headers {
		httpReq.Header.Set(name, value)
	}
	c.h.dispatcher.ServeHTTP(w, httpReq)
	return nil
}

// do sends req and decodes a successful JSON answer into into (when not nil). An error answer
// is an *mcpAPIError. It returns the answer's headers.
func (c *mcpCall) do(req apiRequest, into interface{}) (http.Header, error) {
	w := newMCPResponseWriter()
	if err := c.serve(c.ctx, req, w); err != nil {
		return nil, err
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.status >= 400 {
		return w.header, mcpAPIErrorFrom(w)
	}
	if into != nil {
		if err := json.Unmarshal(w.body.Bytes(), into); err != nil {
			return w.header, fmt.Errorf("decode the %s %s answer: %w", req.method, req.path, err)
		}
	}
	return w.header, nil
}

// Argument readers. A missing required argument fails the call before any request is sent.

func mcpRequireString(args map[string]interface{}, name string) (string, error) {
	value, ok := args[name].(string)
	if !ok || value == "" {
		return "", &ValidationError{Message: name + " is required"}
	}
	return value, nil
}

// mcpOptionalString is the argument as a string; "" when it was not given or is empty.
func mcpOptionalString(args map[string]interface{}, name string) string {
	switch value := args[name].(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		return fmt.Sprint(int64(value))
	default:
		return fmt.Sprint(value)
	}
}

// mcpOptionalNumber is the argument as a number; ok is false when it was not given.
func mcpOptionalNumber(args map[string]interface{}, name string) (value float64, ok bool, err error) {
	switch v := args[name].(type) {
	case nil:
		return 0, false, nil
	case float64:
		value = v
	case string:
		if v == "" {
			return 0, false, nil
		}
		if _, scanErr := fmt.Sscan(v, &value); scanErr != nil {
			return 0, false, &ValidationError{Message: name + " must be a number"}
		}
	default:
		return 0, false, &ValidationError{Message: name + " must be a number"}
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false, &ValidationError{Message: name + " must be a number"}
	}
	return value, true, nil
}

// mcpOptionalList is the argument as a list of strings (an array, or a comma-separated string).
func mcpOptionalList(args map[string]interface{}, name string) ([]string, bool) {
	var items []string
	switch v := args[name].(type) {
	case nil:
		return nil, false
	case []interface{}:
		for _, item := range v {
			items = append(items, fmt.Sprint(item))
		}
	default:
		items = strings.Split(fmt.Sprint(v), ",")
	}
	out := []string{}
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out, true
}
