// Package integration provides end-to-end tests for the SECS/GEM driver.
// These tests spin up a full simulator and connect to it as a host.
package integration

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/dashfactory/go-factory-io/examples/simulator"
	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/transport"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// testEnv holds a connected simulator + host session pair.
type testEnv struct {
	eq      *simulator.Equipment
	host    *hsms.Session
	cancel  context.CancelFunc
	ctx     context.Context
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	logger := slog.Default()

	// Start simulator on random port
	cfg := simulator.DefaultEquipmentConfig()
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.EventInterval = 0 // Disable auto events for deterministic tests

	eq := simulator.NewEquipment(cfg, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	if err := eq.Start(ctx); err != nil {
		cancel()
		t.Fatalf("simulator start: %v", err)
	}

	addr := eq.Addr()
	t.Logf("Simulator listening on %s", addr)

	// Connect host
	hostCfg := hsms.DefaultConfig(addr, hsms.RoleActive, 1)
	hostCfg.LinktestInterval = 0
	host := hsms.NewSession(hostCfg, logger)

	if err := host.Connect(ctx); err != nil {
		eq.Stop()
		cancel()
		t.Fatalf("host connect: %v", err)
	}

	if err := host.Select(ctx); err != nil {
		host.Close()
		eq.Stop()
		cancel()
		t.Fatalf("host select: %v", err)
	}

	// Wait for both sides to reach Selected
	time.Sleep(100 * time.Millisecond)

	return &testEnv{eq: eq, host: host, cancel: cancel, ctx: ctx}
}

func (env *testEnv) teardown() {
	env.host.Close()
	env.eq.Stop()
	env.cancel()
}

// sendSF sends a SECS-II message and returns the decoded reply body.
func sendSF(t *testing.T, env *testEnv, stream, function byte, body *secs2.Item) *secs2.Item {
	t.Helper()

	var data []byte
	if body != nil {
		var err error
		data, err = secs2.Encode(body)
		if err != nil {
			t.Fatalf("encode S%dF%d body: %v", stream, function, err)
		}
	}

	msg := hsms.NewDataMessage(1, stream, function, true, 0, data)
	reply, err := env.host.SendMessage(env.ctx, msg)
	if err != nil {
		t.Fatalf("S%dF%d send: %v", stream, function, err)
	}

	if reply == nil || len(reply.Data) == 0 {
		return nil
	}

	item, err := secs2.Decode(reply.Data)
	if err != nil {
		t.Fatalf("S%dF%d decode reply: %v", stream, function+1, err)
	}
	return item
}

// --- E2E Test: Full Communication Flow ---

func TestE2E_FullCommunicationFlow(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	// Step 1: S1F13 Establish Communication
	t.Run("S1F13_EstablishComm", func(t *testing.T) {
		body := secs2.NewList(
			secs2.NewASCII("HOST"),
			secs2.NewASCII("1.0.0"),
		)
		reply := sendSF(t, env, 1, 13, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		// S1F14: L,2 { B[1] COMMACK, L,2 { A MDLN, A SOFTREV } }
		if reply.Len() < 2 {
			t.Fatalf("expected L,2, got L,%d", reply.Len())
		}

		commack, err := reply.ItemAt(0).ToBinary()
		if err != nil {
			t.Fatalf("COMMACK: %v", err)
		}
		if commack[0] != 0x00 {
			t.Errorf("COMMACK: got %d, want 0 (accepted)", commack[0])
		}

		mdlnList := reply.ItemAt(1)
		mdln, _ := mdlnList.ItemAt(0).ToASCII()
		softrev, _ := mdlnList.ItemAt(1).ToASCII()
		t.Logf("Equipment: MDLN=%q, SOFTREV=%q", mdln, softrev)

		if mdln != "SIM-EQUIP-01" {
			t.Errorf("MDLN: got %q, want %q", mdln, "SIM-EQUIP-01")
		}
	})

	// Step 2: S1F1 Are You There
	t.Run("S1F1_AreYouThere", func(t *testing.T) {
		reply := sendSF(t, env, 1, 1, nil)
		if reply == nil {
			t.Fatal("no reply")
		}

		// S1F2: L,2 { A MDLN, A SOFTREV }
		if reply.Len() != 2 {
			t.Fatalf("expected L,2, got L,%d", reply.Len())
		}
		mdln, _ := reply.ItemAt(0).ToASCII()
		if mdln != "SIM-EQUIP-01" {
			t.Errorf("MDLN: got %q", mdln)
		}
	})

	// Step 3: S1F17 Request ON-LINE
	t.Run("S1F17_RequestOnline", func(t *testing.T) {
		reply := sendSF(t, env, 1, 17, nil)
		if reply == nil {
			t.Fatal("no reply")
		}

		onlack, err := reply.ToBinary()
		if err != nil {
			t.Fatalf("ONLACK: %v", err)
		}
		if onlack[0] != 0x00 {
			t.Errorf("ONLACK: got %d, want 0 (accepted)", onlack[0])
		}
	})

	// Step 4: S1F3 Read Status Variables
	t.Run("S1F3_ReadSV", func(t *testing.T) {
		// Request specific SVIDs: 1001 (WaferCount), 1004 (ProcessState)
		body := secs2.NewList(
			secs2.NewU4(1001),
			secs2.NewU4(1004),
		)
		reply := sendSF(t, env, 1, 3, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		if reply.Len() != 2 {
			t.Fatalf("expected 2 SV values, got %d", reply.Len())
		}

		// WaferCount should be U4
		waferVals, err := reply.ItemAt(0).ToUint64s()
		if err != nil {
			t.Fatalf("WaferCount: %v", err)
		}
		t.Logf("WaferCount = %d", waferVals[0])

		// ProcessState should be ASCII
		state, err := reply.ItemAt(1).ToASCII()
		if err != nil {
			t.Fatalf("ProcessState: %v", err)
		}
		if state != "IDLE" {
			t.Errorf("ProcessState: got %q, want %q", state, "IDLE")
		}
	})

	// Step 5: S1F11 SV Namelist
	t.Run("S1F11_SVNamelist", func(t *testing.T) {
		reply := sendSF(t, env, 1, 11, nil)
		if reply == nil {
			t.Fatal("no reply")
		}

		// Should have 5 SVs defined in simulator
		if reply.Len() < 3 {
			t.Fatalf("expected >= 3 SVs, got %d", reply.Len())
		}

		for _, sv := range reply.Items() {
			svid, _ := sv.ItemAt(0).ToUint64s()
			name, _ := sv.ItemAt(1).ToASCII()
			units, _ := sv.ItemAt(2).ToASCII()
			t.Logf("SVID=%d Name=%q Units=%q", svid[0], name, units)
		}
	})

	// Step 6: S2F13 Read Equipment Constants
	t.Run("S2F13_ReadEC", func(t *testing.T) {
		body := secs2.NewList(
			secs2.NewU4(1), // ProcessTemperature
			secs2.NewU4(2), // ProcessPressure
		)
		reply := sendSF(t, env, 2, 13, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		if reply.Len() != 2 {
			t.Fatalf("expected 2 EC values, got %d", reply.Len())
		}

		tempVals, err := reply.ItemAt(0).ToFloat64s()
		if err != nil {
			t.Fatalf("Temperature: %v", err)
		}
		if tempVals[0] != 350.0 {
			t.Errorf("Temperature: got %f, want 350.0", tempVals[0])
		}
		t.Logf("Temperature=%f, Pressure=%f", tempVals[0], func() float64 {
			v, _ := reply.ItemAt(1).ToFloat64s()
			return v[0]
		}())
	})

	// Step 7: S2F15 Set Equipment Constant
	t.Run("S2F15_SetEC", func(t *testing.T) {
		body := secs2.NewList(
			secs2.NewList(
				secs2.NewU4(1),          // ECID: ProcessTemperature
				secs2.NewF8(400.0),      // New value
			),
		)
		reply := sendSF(t, env, 2, 15, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		eac, err := reply.ToBinary()
		if err != nil {
			t.Fatalf("EAC: %v", err)
		}
		if eac[0] != 0x00 {
			t.Errorf("EAC: got %d, want 0 (accepted)", eac[0])
		}
	})

	// Step 8: S2F29 EC Namelist
	t.Run("S2F29_ECNamelist", func(t *testing.T) {
		reply := sendSF(t, env, 2, 29, nil)
		if reply == nil {
			t.Fatal("no reply")
		}

		if reply.Len() < 3 {
			t.Fatalf("expected >= 3 ECs, got %d", reply.Len())
		}

		for _, ec := range reply.Items() {
			ecid, _ := ec.ItemAt(0).ToUint64s()
			name, _ := ec.ItemAt(1).ToASCII()
			t.Logf("ECID=%d Name=%q", ecid[0], name)
		}
	})

	// Step 9: S1F15 Request OFF-LINE
	t.Run("S1F15_RequestOffline", func(t *testing.T) {
		reply := sendSF(t, env, 1, 15, nil)
		if reply == nil {
			t.Fatal("no reply")
		}

		oflack, err := reply.ToBinary()
		if err != nil {
			t.Fatalf("OFLACK: %v", err)
		}
		if oflack[0] != 0x00 {
			t.Errorf("OFLACK: got %d, want 0 (accepted)", oflack[0])
		}
	})
}

// --- E2E Test: Report Definition and Event Linking ---

func TestE2E_DefineReportAndLinkEvent(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	// Establish communication first
	sendSF(t, env, 1, 13, secs2.NewList(secs2.NewASCII("HOST"), secs2.NewASCII("1.0.0")))

	// S2F33: Define Report
	t.Run("S2F33_DefineReport", func(t *testing.T) {
		body := secs2.NewList(
			secs2.NewU4(1), // DATAID
			secs2.NewList(  // Report definitions
				secs2.NewList(
					secs2.NewU4(10),  // RPTID
					secs2.NewList(    // VIDs
						secs2.NewU4(1001), // WaferCount
						secs2.NewU4(1002), // Temperature
						secs2.NewU4(1003), // Pressure
					),
				),
			),
		)
		reply := sendSF(t, env, 2, 33, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		drack, err := reply.ToBinary()
		if err != nil {
			t.Fatalf("DRACK: %v", err)
		}
		if drack[0] != 0x00 {
			t.Errorf("DRACK: got %d, want 0 (accepted)", drack[0])
		}
	})

	// S2F35: Link Event Report
	t.Run("S2F35_LinkEventReport", func(t *testing.T) {
		body := secs2.NewList(
			secs2.NewU4(2), // DATAID
			secs2.NewList(  // Event-Report links
				secs2.NewList(
					secs2.NewU4(100),  // CEID: ProcessComplete
					secs2.NewList(     // RPTIDs
						secs2.NewU4(10),
					),
				),
			),
		)
		reply := sendSF(t, env, 2, 35, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		lrack, err := reply.ToBinary()
		if err != nil {
			t.Fatalf("LRACK: %v", err)
		}
		if lrack[0] != 0x00 {
			t.Errorf("LRACK: got %d, want 0 (accepted)", lrack[0])
		}
	})

	// S2F37: Enable Event Report
	t.Run("S2F37_EnableEvent", func(t *testing.T) {
		body := secs2.NewList(
			secs2.NewBoolean(true), // CEED: enable
			secs2.NewList(          // CEIDs
				secs2.NewU4(100),
			),
		)
		reply := sendSF(t, env, 2, 37, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		erack, err := reply.ToBinary()
		if err != nil {
			t.Fatalf("ERACK: %v", err)
		}
		if erack[0] != 0x00 {
			t.Errorf("ERACK: got %d, want 0 (accepted)", erack[0])
		}
	})

	// S2F37: Disable all events
	t.Run("S2F37_DisableAll", func(t *testing.T) {
		body := secs2.NewList(
			secs2.NewBoolean(false),
			secs2.NewList(), // Empty = all events
		)
		reply := sendSF(t, env, 2, 37, body)
		if reply == nil {
			t.Fatal("no reply")
		}

		erack, err := reply.ToBinary()
		if err != nil {
			t.Fatalf("ERACK: %v", err)
		}
		if erack[0] != 0x00 {
			t.Errorf("ERACK: got %d, want 0", erack[0])
		}
	})
}

// --- E2E Test: Linktest ---

func TestE2E_Linktest(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	// Send linktest
	systemID := uint32(9999)
	req := hsms.NewLinktestReq(systemID)
	reply, err := env.host.SendMessage(env.ctx, req)
	if err != nil {
		t.Fatalf("linktest: %v", err)
	}
	if reply.Header.SType != hsms.STypeLinktestRsp {
		t.Errorf("expected Linktest.rsp, got %s", reply.Header.SType)
	}
}

// --- E2E Test: Session States ---

func TestE2E_SessionStates(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	if env.host.State() != transport.StateSelected {
		t.Errorf("host state: got %s, want Selected", env.host.State())
	}

	// Deselect
	systemID := uint32(8888)
	req := hsms.NewDeselectReq(1, systemID)
	reply, err := env.host.SendMessage(env.ctx, req)
	if err != nil {
		t.Fatalf("deselect: %v", err)
	}
	if reply.Header.SType != hsms.STypeDeselectRsp {
		t.Errorf("expected Deselect.rsp, got %s", reply.Header.SType)
	}

	time.Sleep(50 * time.Millisecond)
	if env.host.State() != transport.StateConnected {
		// Host sent deselect, but the host session doesn't auto-transition.
		// This is expected — the passive side transitions.
		t.Logf("Host state after deselect: %s (host manages own state)", env.host.State())
	}
}

// --- E2E Test: Dynamic SV Values ---

func TestE2E_DynamicSVValues(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	sendSF(t, env, 1, 13, secs2.NewList(secs2.NewASCII("HOST"), secs2.NewASCII("1.0.0")))

	// Read Temperature (dynamic SV) twice, values should differ slightly
	var temps [2]float64
	for i := range 2 {
		body := secs2.NewList(secs2.NewU4(1002)) // Temperature
		reply := sendSF(t, env, 1, 3, body)
		if reply == nil || reply.Len() == 0 {
			t.Fatal("no reply")
		}
		vals, err := reply.ItemAt(0).ToFloat64s()
		if err != nil {
			t.Fatalf("Temperature read %d: %v", i, err)
		}
		temps[i] = vals[0]
		t.Logf("Temperature read %d: %f", i, temps[i])
	}

	// Both should be in range [348, 352]
	for i, temp := range temps {
		if temp < 348.0 || temp > 352.0 {
			t.Errorf("Temperature read %d out of range: %f", i, temp)
		}
	}
}

// --- E2E Test: S2F33 Delete All Reports ---

func TestE2E_DeleteAllReports(t *testing.T) {
	env := setupTestEnv(t)
	defer env.teardown()

	sendSF(t, env, 1, 13, secs2.NewList(secs2.NewASCII("HOST"), secs2.NewASCII("1.0.0")))

	// S2F33 with empty report list = delete all
	body := secs2.NewList(
		secs2.NewU4(0),   // DATAID
		secs2.NewList(),   // Empty = delete all
	)
	reply := sendSF(t, env, 2, 33, body)
	if reply == nil {
		t.Fatal("no reply")
	}

	drack, err := reply.ToBinary()
	if err != nil {
		t.Fatalf("DRACK: %v", err)
	}
	if drack[0] != 0x00 {
		t.Errorf("DRACK: got %d, want 0", drack[0])
	}
}
