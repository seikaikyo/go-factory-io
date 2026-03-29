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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcapi "github.com/dashfactory/go-factory-io/api/grpc"
	"github.com/dashfactory/go-factory-io/api/rest"
	"github.com/dashfactory/go-factory-io/examples/simulator"
	mqttbridge "github.com/dashfactory/go-factory-io/pkg/bridge/mqtt"
	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/metrics"
	"github.com/dashfactory/go-factory-io/pkg/security"
	"github.com/dashfactory/go-factory-io/pkg/studio"
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
	case "studio":
		runStudio(logger)
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
  studio      Launch SECSGEM Studio (integrated simulator + validator web UI)
  version     Print version information

`)
}

func runSimulator(logger *slog.Logger) {
	fs := flag.NewFlagSet("simulate", flag.ExitOnError)
	addr := fs.String("addr", ":5000", "HSMS listen address")
	apiAddr := fs.String("api", ":8080", "REST API listen address")
	sessionID := fs.Int("session", 1, "Session ID")
	model := fs.String("model", "SIM-EQUIP-01", "Equipment model name")
	version := fs.String("version", "1.0.0", "Software revision")
	eventInterval := fs.Duration("event-interval", 5*time.Second, "Event generation interval (0 to disable)")

	// Phase 4: Edge integration flags
	mqttBroker := fs.String("mqtt-broker", "", "MQTT broker URL (e.g., tcp://localhost:1883)")
	mqttPrefix := fs.String("mqtt-prefix", "", "MQTT topic prefix (default: factory/{model})")
	mqttQoS := fs.Int("mqtt-qos", 1, "MQTT QoS level (0, 1, or 2)")
	grpcAddr := fs.String("grpc-addr", "", "gRPC listen address (e.g., :50051)")
	webhookURL := fs.String("webhook-url", "", "Security event webhook URL")
	syslogAddr := fs.String("syslog-addr", "", "Syslog server address (e.g., syslog.local:514)")

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

	// Security auditor for webhook/syslog
	auditor := security.NewAuditor(logger)

	// Phase 4: MQTT bridge
	if *mqttBroker != "" {
		prefix := *mqttPrefix
		if prefix == "" {
			prefix = "factory/" + *model
		}
		mqttCfg := mqttbridge.DefaultConfig(*mqttBroker, prefix)
		mqttCfg.QoS = byte(*mqttQoS)
		bridge := mqttbridge.NewBridge(mqttCfg, logger)
		if err := bridge.Connect(); err != nil {
			logger.Error("MQTT connect failed", "error", err)
		} else {
			bridge.AttachToHandler(eq.Handler())
			defer bridge.Close()
			logger.Info("MQTT bridge active", "broker", *mqttBroker, "prefix", prefix)
		}
	}

	// Phase 4: Security event webhook
	if *webhookURL != "" {
		handler := security.NewWebhookHandler(security.WebhookConfig{URL: *webhookURL}, logger)
		auditor.OnEvent(handler)
		logger.Info("Security webhook active", "url", *webhookURL)
	}

	// Phase 4: Syslog sink
	if *syslogAddr != "" {
		handler, err := security.NewSyslogHandler(security.SyslogConfig{
			Network: "udp",
			Address: *syslogAddr,
		}, logger)
		if err != nil {
			logger.Error("Syslog handler failed", "error", err)
		} else {
			auditor.OnEvent(handler)
			logger.Info("Syslog sink active", "address", *syslogAddr)
		}
	}

	eq.Handler().SetAuditor(auditor)

	// Phase 4: gRPC server
	var grpcServer *grpcapi.Server
	if *grpcAddr != "" {
		grpcServer = grpcapi.NewServer(eq.Session(), eq.Handler(), logger, "")
		go func() {
			if err := grpcServer.Serve(*grpcAddr); err != nil {
				logger.Error("gRPC server error", "error", err)
			}
		}()
		logger.Info("gRPC server listening", "address", *grpcAddr)
	}

	// Start REST API + Prometheus metrics
	apiServer := rest.NewServer(eq.Session(), eq.Handler(), logger)
	collector := metrics.NewCollector(*model)

	mux := http.NewServeMux()
	mux.Handle("/", apiServer.Handler())
	mux.Handle("/metrics", collector.Handler())

	httpSrv := &http.Server{Addr: *apiAddr, Handler: mux}
	go func() {
		logger.Info("REST API + metrics listening", "address", *apiAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("REST API error", "error", err)
		}
	}()

	logger.Info("Simulator running",
		"hsms", eq.Addr(),
		"api", *apiAddr,
		"metrics", *apiAddr+"/metrics",
	)
	logger.Info("Press Ctrl+C to stop")

	waitForSignal(cancel)
	if grpcServer != nil {
		grpcServer.Stop()
	}
	httpSrv.Shutdown(context.Background())
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

func runStudio(logger *slog.Logger) {
	fs := flag.NewFlagSet("studio", flag.ExitOnError)
	port := fs.Int("port", 8080, "Web UI listen port")
	equipAddr := fs.String("equipment-addr", "", "External equipment address (default: embedded simulator)")
	sessionID := fs.Int("session", 1, "Session ID")
	fs.Parse(os.Args[2:])

	cfg := studio.Config{
		EquipmentAddr: *equipAddr,
		SessionID:     uint16(*sessionID),
	}

	srv := studio.NewServer(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start embedded equipment if no external address
	addr := *equipAddr
	if addr == "" {
		var err error
		addr, err = srv.StartEquipment(ctx)
		if err != nil {
			logger.Error("Failed to start embedded equipment", "error", err)
			os.Exit(1)
		}
		// Wait for equipment to be ready
		time.Sleep(100 * time.Millisecond)
	}

	// Connect host to equipment
	if err := srv.ConnectHost(ctx, addr); err != nil {
		logger.Error("Failed to connect host", "error", err)
		os.Exit(1)
	}

	listenAddr := fmt.Sprintf(":%d", *port)
	httpSrv := &http.Server{Addr: listenAddr, Handler: srv.Handler()}
	go func() {
		logger.Info("SECSGEM Studio running",
			"url", fmt.Sprintf("http://localhost:%d", *port),
			"equipment", addr,
		)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Studio HTTP error", "error", err)
		}
	}()

	logger.Info("Press Ctrl+C to stop")
	waitForSignal(cancel)
	httpSrv.Shutdown(context.Background())
	srv.StopAll()
}

// Ensure packages are used
var _ = gem.NewStateMachine
