// Package simulator provides a simple SECS/GEM equipment simulator for testing.
// It simulates a passive equipment endpoint that responds to standard GEM messages.
package simulator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// Equipment simulates a semiconductor equipment with GEM compliance.
type Equipment struct {
	session *hsms.Session
	handler *gem.Handler
	logger  *slog.Logger
	config  EquipmentConfig
}

// EquipmentConfig defines the simulated equipment parameters.
type EquipmentConfig struct {
	// ListenAddress is the TCP address to listen on (e.g., ":5000").
	ListenAddress string

	// SessionID is the HSMS session ID.
	SessionID uint16

	// ModelName is the equipment model (MDLN).
	ModelName string

	// SoftwareRevision is the software version (SOFTREV).
	SoftwareRevision string

	// EventInterval is how often to generate simulated events.
	// Set to 0 to disable automatic events.
	EventInterval time.Duration
}

// DefaultEquipmentConfig returns a sensible default configuration.
func DefaultEquipmentConfig() EquipmentConfig {
	return EquipmentConfig{
		ListenAddress:    ":5000",
		SessionID:        1,
		ModelName:        "SIM-EQUIP-01",
		SoftwareRevision: "1.0.0",
		EventInterval:    5 * time.Second,
	}
}

// NewEquipment creates a new simulated equipment.
func NewEquipment(cfg EquipmentConfig, logger *slog.Logger) *Equipment {
	if logger == nil {
		logger = slog.Default()
	}

	hsmsCfg := hsms.DefaultConfig(cfg.ListenAddress, hsms.RolePassive, cfg.SessionID)
	hsmsCfg.LinktestInterval = 0 // Host is responsible for linktest

	session := hsms.NewSession(hsmsCfg, logger)
	handler := gem.NewHandler(session, cfg.SessionID, cfg.ModelName, cfg.SoftwareRevision, logger)

	eq := &Equipment{
		session: session,
		handler: handler,
		logger:  logger,
		config:  cfg,
	}

	eq.registerVariables()
	eq.registerEvents()

	return eq
}

// Session returns the underlying HSMS session.
func (eq *Equipment) Session() *hsms.Session {
	return eq.session
}

// Handler returns the GEM handler.
func (eq *Equipment) Handler() *gem.Handler {
	return eq.handler
}

func (eq *Equipment) registerVariables() {
	vars := eq.handler.Variables()

	// Equipment Constants
	vars.DefineEC(&gem.EquipmentConstant{
		ECID:  1,
		Name:  "ProcessTemperature",
		Value: float64(350.0),
		Units: "C",
	})
	vars.DefineEC(&gem.EquipmentConstant{
		ECID:  2,
		Name:  "ProcessPressure",
		Value: float64(760.0),
		Units: "Torr",
	})
	vars.DefineEC(&gem.EquipmentConstant{
		ECID:  3,
		Name:  "ProcessTime",
		Value: uint32(120),
		Units: "sec",
	})

	// Status Variables
	vars.DefineSV(&gem.StatusVariable{
		SVID:  1001,
		Name:  "WaferCount",
		Value: uint32(0),
		Units: "pcs",
	})
	vars.DefineSVDynamic(1002, "Temperature", "C", func() interface{} {
		return float64(348.0 + rand.Float64()*4.0)
	})
	vars.DefineSVDynamic(1003, "Pressure", "Torr", func() interface{} {
		return float64(758.0 + rand.Float64()*4.0)
	})
	vars.DefineSV(&gem.StatusVariable{
		SVID:  1004,
		Name:  "ProcessState",
		Value: "IDLE",
	})
	vars.DefineSVDynamic(1005, "Uptime", "sec", func() interface{} {
		return uint32(time.Now().Unix() % 86400)
	})
}

func (eq *Equipment) registerEvents() {
	events := eq.handler.Events()

	events.DefineEvent(100, "ProcessComplete")
	events.DefineEvent(200, "LotComplete")
	events.DefineEvent(300, "AlarmSet")
	events.DefineEvent(400, "AlarmCleared")

	// Pre-define reports
	events.DefineReport(1, []uint32{1001, 1002, 1003, 1004}) // Process status
	events.DefineReport(2, []uint32{1001})                     // Wafer count only

	// Link events to reports
	events.LinkEventReport(100, []uint32{1})
	events.LinkEventReport(200, []uint32{2})
}

// Start begins listening for connections and processing messages.
func (eq *Equipment) Start(ctx context.Context) error {
	eq.logger.Info("Simulator starting",
		"address", eq.config.ListenAddress,
		"model", eq.config.ModelName,
		"version", eq.config.SoftwareRevision,
	)

	if err := eq.session.Connect(ctx); err != nil {
		return fmt.Errorf("simulator: connect: %w", err)
	}

	// Message processing loop
	go eq.messageLoop(ctx)

	// Event generation loop
	if eq.config.EventInterval > 0 {
		go eq.eventLoop(ctx)
	}

	return nil
}

// Addr returns the listener address.
func (eq *Equipment) Addr() string {
	addr := eq.session.Addr()
	if addr != nil {
		return addr.String()
	}
	return eq.config.ListenAddress
}

// Stop shuts down the simulator.
func (eq *Equipment) Stop() error {
	return eq.session.Close()
}

func (eq *Equipment) messageLoop(ctx context.Context) {
	for {
		msg, err := eq.session.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			eq.logger.Error("Simulator receive error", "error", err)
			return
		}

		if err := eq.handler.HandleMessage(ctx, msg); err != nil {
			eq.logger.Error("Simulator handle error",
				"stream", msg.Header.Stream,
				"function", msg.Header.Function,
				"error", err,
			)
		}
	}
}

func (eq *Equipment) eventLoop(ctx context.Context) {
	ticker := time.NewTicker(eq.config.EventInterval)
	defer ticker.Stop()

	var dataID uint32
	waferCount := uint32(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !eq.handler.State().IsCommunicating() {
				continue
			}

			// Increment wafer count
			waferCount++
			eq.handler.Variables().SetSV(1001, waferCount)

			// Send ProcessComplete event
			dataID++
			if err := eq.handler.SendEvent(ctx, dataID, 100); err != nil {
				eq.logger.Debug("Event send skipped", "error", err)
			}
		}
	}
}
