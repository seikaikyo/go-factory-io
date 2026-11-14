package gem

import (
	"context"
	"fmt"
	"testing"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/security"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// handleNoPanic runs HandleMessage and converts a panic into a test failure,
// so one malformed case does not abort the rest of the table.
func handleNoPanic(t *testing.T, h *Handler, msg *hsms.Message) (err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("S%dF%d panicked: %v", msg.Header.Stream, msg.Header.Function, r)
		}
	}()
	return h.HandleMessage(context.Background(), msg)
}

func mustEncode(t *testing.T, it *secs2.Item) []byte {
	t.Helper()
	data, err := secs2.Encode(it)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return data
}

// TestHandlerMalformedBodiesDoNotPanic covers bodies that decode cleanly but
// carry empty ID items or non-list containers where the handler expects a
// list. Every case used to index [0] of an empty slice or call a method on
// the nil returned by ItemAt.
func TestHandlerMalformedBodiesDoNotPanic(t *testing.T) {
	emptyU4 := secs2.NewU4()
	ascii2 := secs2.NewASCII("ab")

	cases := []struct {
		name             string
		stream, function byte
		body             *secs2.Item
	}{
		{"S1F3 empty SVID", 1, 3, secs2.NewList(emptyU4)},
		{"S1F3 empty list as SVID", 1, 3, secs2.NewList(secs2.NewList())},
		{"S1F3 non-list body", 1, 3, ascii2},
		{"S2F13 empty ECID", 2, 13, secs2.NewList(emptyU4)},
		{"S2F13 non-list body", 2, 13, ascii2},
		{"S2F15 empty ECID", 2, 15, secs2.NewList(secs2.NewList(emptyU4, secs2.NewU4(1)))},
		{"S2F15 non-list pair", 2, 15, secs2.NewList(ascii2)},
		{"S2F33 empty RPTID", 2, 33, secs2.NewList(secs2.NewU4(1),
			secs2.NewList(secs2.NewList(emptyU4, secs2.NewList(secs2.NewU4(1)))))},
		{"S2F33 empty VID", 2, 33, secs2.NewList(secs2.NewU4(1),
			secs2.NewList(secs2.NewList(secs2.NewU4(1), secs2.NewList(emptyU4))))},
		{"S2F33 non-list body", 2, 33, ascii2},
		{"S2F33 non-list report", 2, 33, secs2.NewList(secs2.NewU4(1), secs2.NewList(ascii2))},
		{"S2F35 empty CEID", 2, 35, secs2.NewList(secs2.NewU4(1),
			secs2.NewList(secs2.NewList(emptyU4, secs2.NewList())))},
		{"S2F35 empty RPTID", 2, 35, secs2.NewList(secs2.NewU4(1),
			secs2.NewList(secs2.NewList(secs2.NewU4(100), secs2.NewList(emptyU4))))},
		{"S2F35 non-list body", 2, 35, ascii2},
		{"S2F35 non-list link", 2, 35, secs2.NewList(secs2.NewU4(1), secs2.NewList(ascii2))},
		{"S2F37 empty CEID", 2, 37, secs2.NewList(secs2.NewBoolean(true), secs2.NewList(emptyU4))},
		{"S2F37 non-list body", 2, 37, ascii2},
		{"S2F41 non-list body", 2, 41, ascii2},
		{"S2F41 non-list parameter", 2, 41, secs2.NewList(secs2.NewASCII("START"), secs2.NewList(ascii2))},
		{"S5F3 empty ALID", 5, 3, secs2.NewList(secs2.NewBinary([]byte{0x80}), emptyU4)},
		{"S5F3 non-list body", 5, 3, ascii2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(t)
			h.SetPolicy(security.FullAccessPolicy())
			msg := hsms.NewDataMessage(1, tc.stream, tc.function, false, 1, mustEncode(t, tc.body))
			_ = handleNoPanic(t, h, msg)
		})
	}
}

// TestS1F3EmptySVIDRepliesZeroLength pins the reply for an SVID item with no
// value: SEMI E5 answers an SVID the equipment cannot resolve with a
// zero-length item, which is what an unknown SVID already gets.
func TestS1F3EmptySVIDRepliesZeroLength(t *testing.T) {
	h := newTestHandler(t)
	h.Variables().DefineSV(&StatusVariable{SVID: 1001, Name: "WaferCount", Value: uint32(7)})

	body := secs2.NewList(secs2.NewU4(), secs2.NewU4(1001))
	msg := hsms.NewDataMessage(1, 1, 3, true, 1, mustEncode(t, body))

	var reply *secs2.Item
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("handleS1F3 panicked: %v", r)
			}
		}()
		reply, err = h.handleS1F3(msg)
	}()
	if err != nil {
		t.Fatalf("handleS1F3: %v", err)
	}
	if reply.Len() != 2 {
		t.Fatalf("reply has %d items, want 2", reply.Len())
	}
	if first := reply.ItemAt(0); first.Format() != secs2.FormatList || first.Len() != 0 {
		t.Errorf("empty SVID answered with %s, want L[0]", first)
	}
	if got, _ := reply.ItemAt(1).ToUint64s(); len(got) != 1 || got[0] != 7 {
		t.Errorf("second SVID answered with %v, want 7", got)
	}
}

// TestS2F13EmptyECIDRepliesZeroLength mirrors the S1F3 case for S2F13.
func TestS2F13EmptyECIDRepliesZeroLength(t *testing.T) {
	h := newTestHandler(t)
	body := secs2.NewList(secs2.NewU4())
	msg := hsms.NewDataMessage(1, 2, 13, true, 1, mustEncode(t, body))

	var reply *secs2.Item
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("handleS2F13 panicked: %v", r)
			}
		}()
		reply, err = h.handleS2F13(msg)
	}()
	if err != nil {
		t.Fatalf("handleS2F13: %v", err)
	}
	if reply.Len() != 1 || reply.ItemAt(0).Format() != secs2.FormatList || reply.ItemAt(0).Len() != 0 {
		t.Errorf("reply %s, want L[1] { L[0] }", reply)
	}
}

// TestHandleMessageRecoversCustomHandlerPanic checks that a panic inside a
// message handler is turned into an error and closes only the session that
// carried the message, instead of ending the process.
func TestHandleMessageRecoversCustomHandlerPanic(t *testing.T) {
	h := newTestHandler(t)
	h.SetPolicy(security.FullAccessPolicy())
	h.OnMessage(99, 1, func(ctx context.Context, msg *hsms.Message) (*secs2.Item, error) {
		panic(fmt.Sprintf("boom S%dF%d", msg.Header.Stream, msg.Header.Function))
	})

	msg := hsms.NewDataMessage(1, 99, 1, false, 1, nil)
	err := handleNoPanic(t, h, msg)
	if err == nil {
		t.Fatal("HandleMessage returned nil after a handler panic")
	}

	select {
	case <-h.session.Done():
	default:
		t.Error("session left open after a handler panic")
	}
}
