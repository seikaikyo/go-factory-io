package studio

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/simulator"
)

// rawUpgradeStatus sends a WebSocket upgrade request with the given Host and
// Origin straight over TCP and returns the status line. The Go HTTP client
// always derives Host from the URL, so it cannot express a request whose Host
// names another site while the connection goes to 127.0.0.1.
func rawUpgradeStatus(t *testing.T, tsURL, host, origin string) string {
	t.Helper()
	addr := strings.TrimPrefix(tsURL, "http://")
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	req := "GET /ws HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Origin: " + origin + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	return strings.TrimSpace(line)
}

// TestLoopbackOnlyChecksHost covers a loopback-bound studio without a token.
// Origin and Host both come from the page, so a page that re-points its own
// hostname at 127.0.0.1 passes the same-origin check; the Host header must
// therefore name loopback before the socket is granted control.
func TestLoopbackOnlyChecksHost(t *testing.T) {
	srv := NewServer(Config{SessionID: 1, LoopbackOnly: true}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))

	foreign := "rebind.example:" + port
	if got := rawUpgradeStatus(t, ts.URL, foreign, "http://"+foreign); !strings.Contains(got, " 403 ") {
		t.Errorf("upgrade with foreign Host: %q, want 403", got)
	}
	for _, host := range []string{"127.0.0.1:" + port, "localhost:" + port} {
		if got := rawUpgradeStatus(t, ts.URL, host, "http://"+host); !strings.Contains(got, " 101 ") {
			t.Errorf("upgrade with Host %s: %q, want 101", host, got)
		}
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/trace", nil)
	req.Host = foreign
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("/api/trace with foreign Host: %d, want 403", resp.StatusCode)
	}
}

// TestTraceIsBounded checks that the trace keeps a bounded number of
// entries and bytes, dropping the oldest, and truncates a single oversized
// body.
func TestTraceIsBounded(t *testing.T) {
	s := NewServer(Config{SessionID: 1}, nil)

	body := secs2.NewASCII(strings.Repeat("x", 20<<10))
	for i := 0; i < 1200; i++ {
		s.onMessage(simulator.DirTX, 1, 1, body, nil)
	}

	s.traceMu.Lock()
	n := len(s.trace)
	total := 0
	for _, e := range s.trace {
		total += len(e.BodySML)
	}
	firstID, lastID := s.trace[0].ID, s.trace[n-1].ID
	s.traceMu.Unlock()

	if n > 500 {
		t.Errorf("trace holds %d entries, want at most 500", n)
	}
	if total > 4<<20 {
		t.Errorf("trace holds %d bytes of SML, want at most %d", total, 4<<20)
	}
	if lastID != 1200 {
		t.Errorf("newest entry ID %d, want 1200", lastID)
	}
	if firstID != lastID-uint64(n)+1 {
		t.Errorf("entries are not the newest contiguous run: first %d, last %d, count %d", firstID, lastID, n)
	}

	big := secs2.NewASCII(strings.Repeat("y", 200<<10))
	s.onMessage(simulator.DirRX, 1, 2, big, nil)
	s.traceMu.Lock()
	last := s.trace[len(s.trace)-1]
	s.traceMu.Unlock()
	if len(last.BodySML) > 64<<10+64 {
		t.Errorf("single entry holds %d bytes of SML, want it truncated near 64 KB", len(last.BodySML))
	}
}

// wsDialLocal opens the studio socket and drains the status frame sent on
// connect.
func wsDialLocal(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	c, _, err := dialWS(t, ts.URL, ts.URL, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return c
}

func wsWrite(ctx context.Context, c *websocket.Conn, msgType string, data string) error {
	payload, _ := json.Marshal(WSMessage{Type: msgType, Data: json.RawMessage(data)})
	return c.Write(ctx, websocket.MessageText, payload)
}

// TestWSMessageSizeLimit checks that one oversized frame closes the socket.
func TestWSMessageSizeLimit(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	c := wsDialLocal(t, ts)
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	big := `"` + strings.Repeat("A", 20<<10) + `"`
	_ = wsWrite(ctx, c, "send", `{"body":`+big+`}`)
	_ = wsWrite(ctx, c, "get_state", "")

	_, _, err := c.Read(ctx)
	if err == nil {
		t.Fatal("socket still served after a 20 KB frame; want it closed at the size limit")
	}
	if status := websocket.CloseStatus(err); status != websocket.StatusMessageTooBig {
		t.Errorf("close status %v, want StatusMessageTooBig (err %v)", status, err)
	}
}

// TestWSMessageRateLimit checks that a socket sending far above the per-
// connection rate is closed instead of being served indefinitely.
func TestWSMessageRateLimit(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	c := wsDialLocal(t, ts)
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const sent = 200
	go func() {
		for i := 0; i < sent; i++ {
			if wsWrite(ctx, c, "get_state", "") != nil {
				return
			}
		}
	}()

	replies := 0
	var readErr error
	for replies < sent {
		if _, _, err := c.Read(ctx); err != nil {
			readErr = err
			break
		}
		replies++
	}
	if readErr == nil {
		t.Fatalf("all %d rapid messages were served; want the socket closed by the rate limit", sent)
	}
	if status := websocket.CloseStatus(readErr); status != websocket.StatusPolicyViolation {
		t.Errorf("close status %v after %d replies, want StatusPolicyViolation (err %v)", status, replies, readErr)
	}
}

// readUntil reads frames until one of the wanted type arrives.
func readUntil(ctx context.Context, t *testing.T, c *websocket.Conn, want string) WSMessage {
	t.Helper()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read while waiting for %q: %v", want, err)
		}
		var m WSMessage
		if json.Unmarshal(data, &m) == nil && m.Type == want {
			return m
		}
	}
}

// TestClearTraceRequiresPrivilege checks that a read-only socket cannot wipe
// the trace other viewers rely on.
func TestClearTraceRequiresPrivilege(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil) // no token, not loopback, not demo
	srv.onMessage(simulator.DirTX, 1, 1, nil, nil)
	srv.onMessage(simulator.DirRX, 1, 2, nil, nil)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	c := wsDialLocal(t, ts)
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = wsWrite(ctx, c, "clear_trace", "")
	_ = wsWrite(ctx, c, "get_state", "")

	status := readUntil(ctx, t, c, "status")
	var data struct {
		TraceCount int `json:"traceCount"`
	}
	json.Unmarshal(status.Data, &data)
	if data.TraceCount != 2 {
		t.Errorf("traceCount %d after a read-only clear_trace, want 2", data.TraceCount)
	}
}

// TestSimulatorDemoRefusesDisconnectFault checks that a demo visitor cannot
// cut the embedded simulator off for good, while the demo keeps working.
func TestSimulatorDemoRefusesDisconnectFault(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := NewServer(Config{SessionID: 1, SimulatorDemo: true}, nil)
	addr, err := srv.StartEquipment(ctx)
	if err != nil {
		t.Fatalf("start equipment: %v", err)
	}
	if err := srv.ConnectHost(ctx, addr); err != nil {
		srv.StopAll()
		t.Fatalf("connect host: %v", err)
	}
	defer srv.StopAll()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	c := wsDialLocal(t, ts)
	defer c.Close(websocket.StatusNormalClosure, "")

	_ = wsWrite(ctx, c, "fault", `{"type":"disconnect"}`)
	reply := readUntil(ctx, t, c, "error")
	if !strings.Contains(string(reply.Data), "disconnect") {
		t.Errorf("error reply should name the refused fault, got %s", reply.Data)
	}

	select {
	case <-srv.host.Session().Done():
		t.Fatal("demo host session was closed by a visitor's disconnect fault")
	case <-time.After(200 * time.Millisecond):
	}

	// The demo still answers.
	_ = wsWrite(ctx, c, "quick_send", `{"name":"are_you_there"}`)
	for {
		m := readUntil(ctx, t, c, "trace")
		var e TraceEntry
		json.Unmarshal(m.Data, &e)
		if e.Direction == "rx" && e.Stream == 1 && e.Function == 2 {
			break
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatal("timed out")
	}
}
