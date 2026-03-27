package gem

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/security"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// Handler processes incoming SECS-II messages and generates replies.
// It ties together the state machine, variable store, event manager,
// alarm manager, and command manager.
type Handler struct {
	logger    *slog.Logger
	session   *hsms.Session
	sessionID uint16
	state     *StateMachine
	vars      *VariableStore
	events    *EventManager
	alarms    *AlarmManager
	commands  *CommandManager
	mdln      string // Model name
	softrev   string // Software revision

	// Security (IEC 62443 FR2)
	policy    *security.SessionPolicy
	auditor   *security.Auditor
	interlock *SafetyInterlock

	// Custom message handlers (for application-specific messages)
	customHandlers map[sfKey]MessageHandlerFunc
}

type sfKey struct {
	stream, function byte
}

// MessageHandlerFunc is a custom message handler.
type MessageHandlerFunc func(ctx context.Context, msg *hsms.Message) (*secs2.Item, error)

// NewHandler creates a GEM message handler.
func NewHandler(session *hsms.Session, sessionID uint16, mdln, softrev string, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		logger:         logger,
		session:        session,
		sessionID:      sessionID,
		state:          NewStateMachine(),
		vars:           NewVariableStore(),
		events:         NewEventManager(),
		alarms:         NewAlarmManager(),
		commands:       NewCommandManager(),
		mdln:           mdln,
		softrev:        softrev,
		customHandlers: make(map[sfKey]MessageHandlerFunc),
	}
}

// State returns the GEM state machine.
func (h *Handler) State() *StateMachine { return h.state }

// Variables returns the variable store.
func (h *Handler) Variables() *VariableStore { return h.vars }

// Events returns the event manager.
func (h *Handler) Events() *EventManager { return h.events }

// Alarms returns the alarm manager.
func (h *Handler) Alarms() *AlarmManager { return h.alarms }

// Commands returns the command manager.
func (h *Handler) Commands() *CommandManager { return h.commands }

// SetPolicy sets the session access control policy (IEC 62443 FR2).
func (h *Handler) SetPolicy(policy *security.SessionPolicy) { h.policy = policy }

// Policy returns the current session policy.
func (h *Handler) Policy() *security.SessionPolicy { return h.policy }

// SetAuditor sets the security event auditor.
func (h *Handler) SetAuditor(auditor *security.Auditor) { h.auditor = auditor }

// SetSafetyInterlock configures the SEMI S2 safety interlock.
func (h *Handler) SetSafetyInterlock(interlock *SafetyInterlock) { h.interlock = interlock }

// SafetyInterlock returns the current safety interlock (may be nil).
func (h *Handler) SafetyInterlock() *SafetyInterlock { return h.interlock }

// OnMessage registers a custom handler for a specific Stream/Function.
func (h *Handler) OnMessage(stream, function byte, fn MessageHandlerFunc) {
	h.customHandlers[sfKey{stream, function}] = fn
}

// HandleMessage processes an incoming HSMS data message and sends a reply.
func (h *Handler) HandleMessage(ctx context.Context, msg *hsms.Message) error {
	if msg.IsControlMessage() {
		return nil // Control messages handled by HSMS session
	}

	s := msg.Header.Stream
	f := msg.Header.Function

	h.logger.Debug("GEM received", "stream", s, "function", f, "systemID", msg.Header.SystemID)

	// RBAC check (IEC 62443 FR2)
	if h.policy != nil && !h.policy.IsAllowed(s, f) {
		source := fmt.Sprintf("session:%d", msg.Header.SessionID)
		h.logger.Warn("GEM message denied by policy", "stream", s, "function", f)
		if h.auditor != nil {
			h.auditor.UnauthorizedMessage(source, s, f)
		}
		// Send S{s}F0 (abort) if WBit is set
		if msg.Header.WBit {
			return h.sendReply(msg, s, 0, nil)
		}
		return nil
	}

	// Check for custom handler first
	if fn, ok := h.customHandlers[sfKey{s, f}]; ok {
		reply, err := fn(ctx, msg)
		if err != nil {
			return fmt.Errorf("gem: custom handler S%dF%d: %w", s, f, err)
		}
		if reply != nil && msg.Header.WBit {
			return h.sendReply(msg, s, f+1, reply)
		}
		return nil
	}

	// Built-in message handlers
	var reply *secs2.Item
	var err error

	switch {
	case s == 1 && f == 1: // S1F1: Are You There
		reply = h.handleS1F1()
	case s == 1 && f == 3: // S1F3: Selected Equipment Status Request
		reply, err = h.handleS1F3(msg)
	case s == 1 && f == 11: // S1F11: SV Namelist Request
		reply, err = h.handleS1F11(msg)
	case s == 1 && f == 13: // S1F13: Establish Communication Request
		reply = h.handleS1F13()
	case s == 1 && f == 15: // S1F15: Request OFF-LINE
		reply = h.handleS1F15()
	case s == 1 && f == 17: // S1F17: Request ON-LINE
		reply = h.handleS1F17()
	case s == 2 && f == 13: // S2F13: Equipment Constant Request
		reply, err = h.handleS2F13(msg)
	case s == 2 && f == 15: // S2F15: New Equipment Constant Send
		reply, err = h.handleS2F15(msg)
	case s == 2 && f == 29: // S2F29: EC Namelist Request
		reply, err = h.handleS2F29(msg)
	case s == 2 && f == 33: // S2F33: Define Report
		reply, err = h.handleS2F33(msg)
	case s == 2 && f == 35: // S2F35: Link Event Report
		reply, err = h.handleS2F35(msg)
	case s == 2 && f == 37: // S2F37: Enable/Disable Event Report
		reply, err = h.handleS2F37(msg)
	case s == 2 && f == 41: // S2F41: Host Command Send (RCMD)
		reply, err = h.handleS2F41(msg)
	case s == 5 && f == 3: // S5F3: Enable/Disable Alarm Send
		reply, err = h.handleS5F3(msg)
	case s == 5 && f == 5: // S5F5: List Alarms Request
		reply = h.handleS5F5()
	case s == 5 && f == 7: // S5F7: List Enabled Alarms Request
		reply = h.handleS5F7()
	default:
		h.logger.Warn("GEM unhandled message", "stream", s, "function", f)
		return nil
	}

	if err != nil {
		return fmt.Errorf("gem: S%dF%d handler: %w", s, f, err)
	}

	if reply != nil && msg.Header.WBit {
		return h.sendReply(msg, s, f+1, reply)
	}
	return nil
}

func (h *Handler) sendReply(req *hsms.Message, stream, function byte, body *secs2.Item) error {
	var data []byte
	if body != nil {
		var err error
		data, err = secs2.Encode(body)
		if err != nil {
			return fmt.Errorf("gem: encode reply: %w", err)
		}
	}
	reply := hsms.NewDataMessage(req.Header.SessionID, stream, function, false, req.Header.SystemID, data)
	_, err := h.session.SendMessage(context.Background(), reply)
	return err
}

// --- S1 Handlers ---

// S1F1 -> S1F2: Are You There / On Line Data
func (h *Handler) handleS1F1() *secs2.Item {
	return secs2.NewList(
		secs2.NewASCII(h.mdln),
		secs2.NewASCII(h.softrev),
	)
}

// S1F3 -> S1F4: Selected Equipment Status
func (h *Handler) handleS1F3(msg *hsms.Message) (*secs2.Item, error) {
	if len(msg.Data) == 0 {
		// Empty request = return all SVs
		svids := h.vars.ListSVIDs()
		items := make([]*secs2.Item, len(svids))
		for i, svid := range svids {
			val, _ := h.vars.GetSV(svid)
			items[i] = valueToItem(val)
		}
		return secs2.NewList(items...), nil
	}

	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	items := make([]*secs2.Item, body.Len())
	for i, child := range body.Items() {
		svids, err := child.ToUint64s()
		if err != nil {
			return nil, fmt.Errorf("SVID %d: %w", i, err)
		}
		val, _ := h.vars.GetSV(uint32(svids[0]))
		items[i] = valueToItem(val)
	}
	return secs2.NewList(items...), nil
}

// S1F11 -> S1F12: Status Variable Namelist
func (h *Handler) handleS1F11(msg *hsms.Message) (*secs2.Item, error) {
	svids := h.vars.ListSVIDs()

	items := make([]*secs2.Item, len(svids))
	for i, svid := range svids {
		info, _ := h.vars.GetSVInfo(svid)
		items[i] = secs2.NewList(
			secs2.NewU4(svid),
			secs2.NewASCII(info.Name),
			secs2.NewASCII(info.Units),
		)
	}
	return secs2.NewList(items...), nil
}

// S1F13 -> S1F14: Establish Communication
func (h *Handler) handleS1F13() *secs2.Item {
	h.state.EnableComm()
	h.state.CommEstablished()

	return secs2.NewList(
		secs2.NewBinary([]byte{0x00}), // COMMACK: 0 = accepted
		secs2.NewList(
			secs2.NewASCII(h.mdln),
			secs2.NewASCII(h.softrev),
		),
	)
}

// S1F15 -> S1F16: Request OFF-LINE
func (h *Handler) handleS1F15() *secs2.Item {
	if err := h.state.GoOffline(); err != nil {
		return secs2.NewBinary([]byte{0x01}) // OFLACK: 1 = rejected
	}
	return secs2.NewBinary([]byte{0x00}) // OFLACK: 0 = accepted
}

// S1F17 -> S1F18: Request ON-LINE
func (h *Handler) handleS1F17() *secs2.Item {
	if err := h.state.GoOnlineRemote(); err != nil {
		return secs2.NewBinary([]byte{0x01}) // ONLACK: 1 = rejected
	}
	return secs2.NewBinary([]byte{0x00}) // ONLACK: 0 = accepted
}

// --- S2 Handlers ---

// S2F13 -> S2F14: Equipment Constant Request
func (h *Handler) handleS2F13(msg *hsms.Message) (*secs2.Item, error) {
	if len(msg.Data) == 0 {
		ecids := h.vars.ListECIDs()
		items := make([]*secs2.Item, len(ecids))
		for i, ecid := range ecids {
			ec, _ := h.vars.GetEC(ecid)
			items[i] = valueToItem(ec.Value)
		}
		return secs2.NewList(items...), nil
	}

	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	items := make([]*secs2.Item, body.Len())
	for i, child := range body.Items() {
		ecids, err := child.ToUint64s()
		if err != nil {
			return nil, fmt.Errorf("ECID %d: %w", i, err)
		}
		ec, ok := h.vars.GetEC(uint32(ecids[0]))
		if ok {
			items[i] = valueToItem(ec.Value)
		} else {
			items[i] = secs2.NewList() // Empty for unknown
		}
	}
	return secs2.NewList(items...), nil
}

// S2F15 -> S2F16: New Equipment Constant Send
func (h *Handler) handleS2F15(msg *hsms.Message) (*secs2.Item, error) {
	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	for _, pair := range body.Items() {
		if pair.Len() < 2 {
			continue
		}
		ecids, err := pair.ItemAt(0).ToUint64s()
		if err != nil {
			continue
		}
		ecid := uint32(ecids[0])
		// For simplicity, store the raw Item value
		h.vars.SetEC(ecid, pair.ItemAt(1))
	}

	return secs2.NewBinary([]byte{0x00}), nil // EAC: 0 = accepted
}

// S2F29 -> S2F30: EC Namelist Request
func (h *Handler) handleS2F29(msg *hsms.Message) (*secs2.Item, error) {
	ecids := h.vars.ListECIDs()

	items := make([]*secs2.Item, len(ecids))
	for i, ecid := range ecids {
		ec, _ := h.vars.GetEC(ecid)
		items[i] = secs2.NewList(
			secs2.NewU4(ecid),
			secs2.NewASCII(ec.Name),
			valueToItem(ec.MinValue),
			valueToItem(ec.MaxValue),
			valueToItem(ec.Value),
			secs2.NewASCII(ec.Units),
		)
	}
	return secs2.NewList(items...), nil
}

// S2F33 -> S2F34: Define Report
func (h *Handler) handleS2F33(msg *hsms.Message) (*secs2.Item, error) {
	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	if body.Len() < 2 {
		return secs2.NewBinary([]byte{0x01}), nil // DRACK: 1 = error
	}

	// body: L,2 { DATAID, L,n { L,2 { RPTID, L,m { VID... } } } }
	reportList := body.ItemAt(1)
	if reportList.Len() == 0 {
		// Empty list = delete all reports
		h.events.DeleteAllReports()
		return secs2.NewBinary([]byte{0x00}), nil
	}

	for _, rptDef := range reportList.Items() {
		if rptDef.Len() < 2 {
			continue
		}
		rptids, err := rptDef.ItemAt(0).ToUint64s()
		if err != nil {
			continue
		}
		rptid := uint32(rptids[0])

		vidList := rptDef.ItemAt(1)
		if vidList.Len() == 0 {
			h.events.DeleteReport(rptid)
			continue
		}

		vids := make([]uint32, vidList.Len())
		for i, vidItem := range vidList.Items() {
			ids, err := vidItem.ToUint64s()
			if err != nil {
				continue
			}
			vids[i] = uint32(ids[0])
		}
		h.events.DefineReport(rptid, vids)
	}

	return secs2.NewBinary([]byte{0x00}), nil // DRACK: 0 = accepted
}

// S2F35 -> S2F36: Link Event Report
func (h *Handler) handleS2F35(msg *hsms.Message) (*secs2.Item, error) {
	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	if body.Len() < 2 {
		return secs2.NewBinary([]byte{0x01}), nil // LRACK: 1 = error
	}

	// body: L,2 { DATAID, L,n { L,2 { CEID, L,m { RPTID... } } } }
	linkList := body.ItemAt(1)
	for _, link := range linkList.Items() {
		if link.Len() < 2 {
			continue
		}
		ceids, err := link.ItemAt(0).ToUint64s()
		if err != nil {
			continue
		}
		ceid := uint32(ceids[0])

		rptList := link.ItemAt(1)
		rptids := make([]uint32, rptList.Len())
		for i, rptItem := range rptList.Items() {
			ids, err := rptItem.ToUint64s()
			if err != nil {
				continue
			}
			rptids[i] = uint32(ids[0])
		}
		h.events.LinkEventReport(ceid, rptids)
	}

	return secs2.NewBinary([]byte{0x00}), nil // LRACK: 0 = accepted
}

// S2F37 -> S2F38: Enable/Disable Event Report
func (h *Handler) handleS2F37(msg *hsms.Message) (*secs2.Item, error) {
	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	if body.Len() < 2 {
		return secs2.NewBinary([]byte{0x01}), nil // ERACK: 1 = error
	}

	// body: L,2 { CEED (bool), L,n { CEID... } }
	enableFlags, err := body.ItemAt(0).ToBooleans()
	if err != nil {
		return secs2.NewBinary([]byte{0x01}), nil
	}
	enabled := len(enableFlags) > 0 && enableFlags[0]

	ceidList := body.ItemAt(1)
	if ceidList.Len() == 0 {
		// Empty list = apply to all events
		h.events.EnableAllEvents(enabled)
	} else {
		for _, ceidItem := range ceidList.Items() {
			ceids, err := ceidItem.ToUint64s()
			if err != nil {
				continue
			}
			h.events.EnableEvent(uint32(ceids[0]), enabled)
		}
	}

	return secs2.NewBinary([]byte{0x00}), nil // ERACK: 0 = accepted
}

// --- S2F41: Remote Command ---

// S2F41 -> S2F42: Host Command Send
func (h *Handler) handleS2F41(msg *hsms.Message) (*secs2.Item, error) {
	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	if body.Len() < 2 {
		return secs2.NewList(
			secs2.NewBinary([]byte{byte(CommandParameterError)}),
			secs2.NewList(),
		), nil
	}

	cmdName, err := body.ItemAt(0).ToASCII()
	if err != nil {
		return secs2.NewList(
			secs2.NewBinary([]byte{byte(CommandParameterError)}),
			secs2.NewList(),
		), nil
	}

	// Parse parameters: L,n { L,2 { A cpname, cpval } }
	var params []CommandParam
	paramList := body.ItemAt(1)
	for _, p := range paramList.Items() {
		if p.Len() < 2 {
			continue
		}
		name, _ := p.ItemAt(0).ToASCII()
		params = append(params, CommandParam{
			Name:  name,
			Value: p.ItemAt(1),
		})
	}

	status := h.commands.Execute(context.Background(), cmdName, params)
	h.logger.Info("RCMD executed", "command", cmdName, "status", status)

	return secs2.NewList(
		secs2.NewBinary([]byte{byte(status)}),
		secs2.NewList(), // Empty CPACK for now
	), nil
}

// --- S5: Alarm Handlers ---

// S5F3 -> S5F4: Enable/Disable Alarm Send
func (h *Handler) handleS5F3(msg *hsms.Message) (*secs2.Item, error) {
	body, err := secs2.Decode(msg.Data)
	if err != nil {
		return nil, err
	}

	if body.Len() < 2 {
		return secs2.NewBinary([]byte{0x01}), nil // ACKC5: 1 = error
	}

	// body: L,2 { ALED (binary: 0=disable, 0x80=enable), ALID }
	aledBytes, err := body.ItemAt(0).ToBinary()
	if err != nil {
		return secs2.NewBinary([]byte{0x01}), nil
	}
	enabled := len(aledBytes) > 0 && aledBytes[0]&0x80 != 0

	alids, err := body.ItemAt(1).ToUint64s()
	if err != nil {
		return secs2.NewBinary([]byte{0x01}), nil
	}

	if err := h.alarms.EnableAlarm(uint32(alids[0]), enabled); err != nil {
		return secs2.NewBinary([]byte{0x01}), nil
	}

	return secs2.NewBinary([]byte{0x00}), nil // ACKC5: 0 = accepted
}

// S5F5 -> S5F6: List Alarms Request
func (h *Handler) handleS5F5() *secs2.Item {
	alarms := h.alarms.ListAlarms()
	items := make([]*secs2.Item, len(alarms))
	for i, a := range alarms {
		alcd := byte(a.Severity)
		if a.State == AlarmSet {
			alcd |= 0x80 // Bit 7 = alarm set
		}
		items[i] = secs2.NewList(
			secs2.NewBinary([]byte{alcd}), // ALCD
			secs2.NewU4(a.ALID),
			secs2.NewASCII(a.Text),
		)
	}
	return secs2.NewList(items...)
}

// S5F7 -> S5F8: List Enabled Alarms Request
func (h *Handler) handleS5F7() *secs2.Item {
	alarms := h.alarms.ListAlarms()
	var items []*secs2.Item
	for _, a := range alarms {
		if !a.Enabled {
			continue
		}
		alcd := byte(a.Severity)
		if a.State == AlarmSet {
			alcd |= 0x80
		}
		items = append(items, secs2.NewList(
			secs2.NewBinary([]byte{alcd}),
			secs2.NewU4(a.ALID),
			secs2.NewASCII(a.Text),
		))
	}
	return secs2.NewList(items...)
}

// --- Alarm Sending ---

// SendAlarm sends an S5F1 alarm report to the host.
func (h *Handler) SendAlarm(ctx context.Context, alid uint32, set bool) error {
	var alarm *Alarm
	var err error
	if set {
		alarm, err = h.alarms.SetAlarm(alid)
	} else {
		alarm, err = h.alarms.ClearAlarm(alid)
	}
	if err != nil {
		return err
	}

	// Safety interlock check (SEMI S2)
	if set && h.interlock != nil {
		h.interlock.Evaluate(alarm)
	}

	if !alarm.Enabled {
		return nil // Silently skip disabled alarms
	}

	alcd := byte(alarm.Severity)
	if set {
		alcd |= 0x80
	}

	body := secs2.NewList(
		secs2.NewBinary([]byte{alcd}), // ALCD
		secs2.NewU4(alarm.ALID),
		secs2.NewASCII(alarm.Text),
	)

	data, err := secs2.Encode(body)
	if err != nil {
		return err
	}

	msg := hsms.NewDataMessage(h.sessionID, 5, 1, true, 0, data)
	_, err = h.session.SendMessage(ctx, msg)
	return err
}

// --- Event Sending ---

// SendEvent sends an S6F11 event report message to the host.
func (h *Handler) SendEvent(ctx context.Context, dataID, ceid uint32) error {
	if !h.events.IsEventEnabled(ceid) {
		return nil // Silently skip disabled events
	}

	reportVIDs, err := h.events.GetEventReportVIDs(ceid)
	if err != nil {
		return err
	}

	// Build report list
	rptItems := make([]*secs2.Item, len(reportVIDs))
	for i, rv := range reportVIDs {
		vidItems := make([]*secs2.Item, len(rv.VIDs))
		for j, vid := range rv.VIDs {
			val, _ := h.vars.GetSV(vid)
			vidItems[j] = valueToItem(val)
		}
		rptItems[i] = secs2.NewList(
			secs2.NewU4(rv.RPTID),
			secs2.NewList(vidItems...),
		)
	}

	body := secs2.NewList(
		secs2.NewU4(dataID),
		secs2.NewU4(ceid),
		secs2.NewList(rptItems...),
	)

	data, err := secs2.Encode(body)
	if err != nil {
		return err
	}

	msg := hsms.NewDataMessage(h.sessionID, 6, 11, true, 0, data)
	_, err = h.session.SendMessage(ctx, msg)
	return err
}

// --- Helpers ---

// valueToItem converts a Go value to a SECS-II Item.
func valueToItem(val interface{}) *secs2.Item {
	if val == nil {
		return secs2.NewList() // Empty list for nil
	}
	switch v := val.(type) {
	case *secs2.Item:
		return v
	case string:
		return secs2.NewASCII(v)
	case int:
		return secs2.NewI4(int32(v))
	case int32:
		return secs2.NewI4(v)
	case int64:
		return secs2.NewI8(v)
	case uint32:
		return secs2.NewU4(v)
	case uint64:
		return secs2.NewU8(v)
	case float32:
		return secs2.NewF4(v)
	case float64:
		return secs2.NewF8(v)
	case bool:
		return secs2.NewBoolean(v)
	case []byte:
		return secs2.NewBinary(v)
	default:
		return secs2.NewASCII(fmt.Sprintf("%v", v))
	}
}
