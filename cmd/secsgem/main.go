// secsgem is a SECS/GEM equipment communication daemon.
//
// Usage:
//
//	secsgem simulate                  - Run equipment simulator
//	secsgem connect <host:port>       - Connect to equipment as host
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dashfactory/go-factory-io/examples/simulator"
	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	switch os.Args[1] {
	case "simulate":
		runSimulator(logger)
	case "connect":
		runConnect(logger)
	case "version":
		fmt.Println("go-factory-io v0.1.0")
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: secsgem <command> [options]

Commands:
  simulate    Run an equipment simulator (passive mode)
  connect     Connect to equipment as host (active mode)
  version     Print version information

`)
}

func runSimulator(logger *slog.Logger) {
	fs := flag.NewFlagSet("simulate", flag.ExitOnError)
	addr := fs.String("addr", ":5000", "Listen address")
	sessionID := fs.Int("session", 1, "Session ID")
	model := fs.String("model", "SIM-EQUIP-01", "Equipment model name")
	version := fs.String("version", "1.0.0", "Software revision")
	eventInterval := fs.Duration("event-interval", 5*time.Second, "Event generation interval (0 to disable)")
	fs.Parse(os.Args[2:])

	cfg := simulator.EquipmentConfig{
		ListenAddress:    *addr,
		SessionID:        uint16(*sessionID),
		ModelName:        *model,
		SoftwareRevision: *version,
		EventInterval:    *eventInterval,
	}

	eq := simulator.NewEquipment(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := eq.Start(ctx); err != nil {
		logger.Error("Failed to start simulator", "error", err)
		os.Exit(1)
	}

	logger.Info("Simulator running", "address", eq.Addr())
	logger.Info("Press Ctrl+C to stop")

	waitForSignal(cancel)
	eq.Stop()
}

func runConnect(logger *slog.Logger) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	sessionID := fs.Int("session", 1, "Session ID")
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: secsgem connect [options] <host:port>\n")
		os.Exit(1)
	}
	address := args[0]

	cfg := hsms.DefaultConfig(address, hsms.RoleActive, uint16(*sessionID))
	session := hsms.NewSession(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("Connecting to equipment", "address", address)

	if err := session.Connect(ctx); err != nil {
		logger.Error("Connection failed", "error", err)
		os.Exit(1)
	}
	defer session.Close()

	if err := session.Select(ctx); err != nil {
		logger.Error("Select failed", "error", err)
		os.Exit(1)
	}

	// Send S1F13 Establish Communication
	body := secs2.NewList(
		secs2.NewASCII("HOST"),
		secs2.NewASCII("1.0.0"),
	)
	data, _ := secs2.Encode(body)
	msg := hsms.NewDataMessage(uint16(*sessionID), 1, 13, true, 0, data)
	reply, err := session.SendMessage(ctx, msg)
	if err != nil {
		logger.Error("S1F13 failed", "error", err)
		os.Exit(1)
	}

	if reply != nil && len(reply.Data) > 0 {
		item, err := secs2.Decode(reply.Data)
		if err == nil {
			logger.Info("S1F14 response", "body", item.String())
		}
	}

	// Send S1F1 Are You There
	_ = sendS1F1(ctx, session, uint16(*sessionID), logger)

	// Read SV names
	_ = sendS1F11(ctx, session, uint16(*sessionID), logger)

	logger.Info("Connected and communicating. Press Ctrl+C to disconnect.")

	// Listen for incoming messages
	go func() {
		for {
			msg, err := session.ReceiveMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("Receive error", "error", err)
				return
			}
			logger.Info("Received message",
				"stream", msg.Header.Stream,
				"function", msg.Header.Function,
				"dataLen", len(msg.Data),
			)
			if len(msg.Data) > 0 {
				if item, err := secs2.Decode(msg.Data); err == nil {
					logger.Info("Message body", "body", item.String())
				}
			}
		}
	}()

	waitForSignal(cancel)
}

func sendS1F1(ctx context.Context, session *hsms.Session, sessionID uint16, logger *slog.Logger) error {
	msg := hsms.NewDataMessage(sessionID, 1, 1, true, 0, nil)
	reply, err := session.SendMessage(ctx, msg)
	if err != nil {
		logger.Error("S1F1 failed", "error", err)
		return err
	}
	if reply != nil && len(reply.Data) > 0 {
		item, err := secs2.Decode(reply.Data)
		if err == nil {
			logger.Info("S1F2 response (Online Data)", "body", item.String())
		}
	}
	return nil
}

func sendS1F11(ctx context.Context, session *hsms.Session, sessionID uint16, logger *slog.Logger) error {
	msg := hsms.NewDataMessage(sessionID, 1, 11, true, 0, nil)
	reply, err := session.SendMessage(ctx, msg)
	if err != nil {
		logger.Error("S1F11 failed", "error", err)
		return err
	}
	if reply != nil && len(reply.Data) > 0 {
		item, err := secs2.Decode(reply.Data)
		if err == nil {
			logger.Info("S1F12 response (SV Namelist)", "body", item.String())
		}
	}
	return nil
}

func waitForSignal(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
}

// Ensure gem package is used (will be used in future host-side handler)
var _ = gem.NewStateMachine
