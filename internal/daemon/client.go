package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client connects to a running daemon over its Unix socket and sends
// newline-delimited JSON messages.
type Client struct {
	socketPath string
	conn       net.Conn
	enc        *json.Encoder
	scanner    *bufio.Scanner
}

// Dial connects to the daemon at socketPath.
// Returns an error immediately if the daemon is not running.
func Dial(socketPath string) (*Client, error) {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), bufio.MaxScanTokenSize)
	return &Client{
		socketPath: socketPath,
		conn:       conn,
		enc:        json.NewEncoder(conn),
		scanner:    scanner,
	}, nil
}

// Send sends a JSON message and reads the JSON response.
// The message must be a map that includes an "op" field.
func (c *Client) Send(msg map[string]any) (map[string]any, error) {
	if err := c.enc.Encode(msg); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("recv: %w", err)
		}
		return nil, fmt.Errorf("recv: connection closed")
	}
	var resp map[string]any
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("recv decode: %w", err)
	}
	return resp, nil
}

// Ping returns true if the daemon is running and responds to a ping.
func (c *Client) Ping() bool {
	resp, err := c.Send(map[string]any{"op": "ping"})
	if err != nil {
		return false
	}
	ok, _ := resp["ok"].(bool)
	return ok
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsDaemonRunning is a helper that dials the socket and pings the daemon.
// It returns true only if the daemon is alive and responsive.
// The connection is closed after the check.
func IsDaemonRunning(socketPath string) bool {
	c, err := Dial(socketPath)
	if err != nil {
		return false
	}
	defer c.Close()
	return c.Ping()
}
