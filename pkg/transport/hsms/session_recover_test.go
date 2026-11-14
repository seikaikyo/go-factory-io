package hsms

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/security"
)

const readLoopPanicChildEnv = "HSMS_READLOOP_PANIC_CHILD"

// TestReadLoopPanicClosesOnlySession checks that a panic raised while the
// read loop handles one message closes that session and leaves the process
// running. The panic is raised from a security event handler, which is
// caller-supplied code the read loop runs synchronously.
//
// An unrecovered panic in a background goroutine ends the whole test binary,
// so the scenario runs in a child process and the parent checks its exit.
func TestReadLoopPanicClosesOnlySession(t *testing.T) {
	if os.Getenv(readLoopPanicChildEnv) == "1" {
		runReadLoopPanicScenario(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestReadLoopPanicClosesOnlySession$", "-test.v")
	cmd.Env = append(os.Environ(), readLoopPanicChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child process failed (%v); a read loop panic must not end the process:\n%s",
			err, tail(string(out), 20))
	}
	if !strings.Contains(string(out), "session closed, process alive") {
		t.Fatalf("child did not reach the end of the scenario:\n%s", tail(string(out), 20))
	}
}

func runReadLoopPanicScenario(t *testing.T) {
	logger := slog.Default()

	auditor := security.NewAuditor(logger)
	auditor.OnEvent(func(security.Event) { panic("event handler failure") })

	passiveCfg := DefaultConfig("127.0.0.1:0", RolePassive, 0x0001)
	passiveCfg.LinktestInterval = 0
	passiveCfg.T7 = 0
	passiveCfg.Auditor = auditor
	passive := NewSession(passiveCfg, logger)
	defer passive.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}

	activeCfg := DefaultConfig(passive.Addr().String(), RoleActive, 0x0001)
	activeCfg.LinktestInterval = 0
	activeCfg.T7 = 0
	active := NewSession(activeCfg, logger)
	defer active.Close()
	if err := active.Connect(ctx); err != nil {
		t.Fatalf("active connect: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// A data message before Select makes the passive read loop record a
	// security event, which runs the panicking handler.
	if err := active.writeMessage(NewDataMessage(0x0001, 1, 1, true, 7, nil)); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case <-passive.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("passive session still open after a read loop panic")
	}
	t.Log("session closed, process alive")
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
