// Package grpcapi provides a gRPC server for SECS/GEM equipment communication.
// Mirrors the REST API endpoints for high-frequency M2M integration.
package grpcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Server implements the gRPC SECSGEMService.
type Server struct {
	handler     *gem.Handler
	session     *hsms.Session
	logger      *slog.Logger
	bearerToken string
	grpcServer  *grpc.Server

	// Event streaming
	streamMu sync.Mutex
	streams  map[chan eventMsg]struct{}
}

type eventMsg struct {
	Type      string
	Timestamp string
	Data      interface{}
}

// NewServer creates a gRPC server for SECS/GEM.
func NewServer(session *hsms.Session, handler *gem.Handler, logger *slog.Logger, bearerToken string) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		handler:     handler,
		session:     session,
		logger:      logger,
		bearerToken: bearerToken,
		streams:     make(map[chan eventMsg]struct{}),
	}

	// Register event hooks for streaming
	handler.OnEventSent(func(dataID, ceid uint32) {
		s.broadcastEvent("collection_event", map[string]interface{}{"dataID": dataID, "ceid": ceid})
	})
	handler.OnAlarmSent(func(alid uint32, set bool, alarm *gem.Alarm) {
		state := "cleared"
		if set {
			state = "set"
		}
		s.broadcastEvent("alarm", map[string]interface{}{
			"alid": alid, "state": state, "text": alarm.Text, "name": alarm.Name,
		})
	})

	return s
}

// Serve starts the gRPC server on the given address.
func (s *Server) Serve(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc: listen %s: %w", addr, err)
	}

	opts := []grpc.ServerOption{}
	if s.bearerToken != "" {
		opts = append(opts,
			grpc.UnaryInterceptor(s.unaryAuthInterceptor()),
			grpc.StreamInterceptor(s.streamAuthInterceptor()),
		)
	}

	s.grpcServer = grpc.NewServer(opts...)
	RegisterSECSGEMServiceServer(s.grpcServer, s)

	s.logger.Info("gRPC server listening", "address", addr)
	return s.grpcServer.Serve(ln)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// --- Auth Interceptors ---

func (s *Server) unaryAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := s.checkAuth(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (s *Server) streamAuthInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := s.checkAuth(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func (s *Server) checkAuth(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization")
	}
	expected := "Bearer " + s.bearerToken
	if values[0] != expected {
		return status.Error(codes.Unauthenticated, "invalid bearer token")
	}
	return nil
}

// --- RPC Implementations ---

func (s *Server) GetStatus(_ context.Context, _ *GetStatusRequest) (*GetStatusResponse, error) {
	st := s.handler.State()
	return &GetStatusResponse{
		CommState:      st.CommState().String(),
		ControlState:   st.ControlState().String(),
		Communicating:  st.IsCommunicating(),
		Online:         st.IsOnline(),
		TransportState: s.session.State().String(),
	}, nil
}

func (s *Server) ListStatusVariables(_ context.Context, _ *ListSVRequest) (*ListSVResponse, error) {
	vars := s.handler.Variables()
	svids := vars.ListSVIDs()

	result := make([]*StatusVariable, 0, len(svids))
	for _, svid := range svids {
		info, _ := vars.GetSVInfo(svid)
		val, _ := vars.GetSV(svid)
		result = append(result, &StatusVariable{
			Svid:  svid,
			Name:  info.Name,
			Units: info.Units,
			Value: toJSON(val),
		})
	}
	return &ListSVResponse{Variables: result}, nil
}

func (s *Server) GetStatusVariable(_ context.Context, req *GetSVRequest) (*GetSVResponse, error) {
	val, ok := s.handler.Variables().GetSV(req.Svid)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "SVID %d not found", req.Svid)
	}
	info, _ := s.handler.Variables().GetSVInfo(req.Svid)
	return &GetSVResponse{
		Variable: &StatusVariable{
			Svid:  req.Svid,
			Name:  info.Name,
			Units: info.Units,
			Value: toJSON(val),
		},
	}, nil
}

func (s *Server) ListEquipmentConstants(_ context.Context, _ *ListECRequest) (*ListECResponse, error) {
	vars := s.handler.Variables()
	ecids := vars.ListECIDs()

	result := make([]*EquipmentConstant, 0, len(ecids))
	for _, ecid := range ecids {
		ec, _ := vars.GetEC(ecid)
		result = append(result, &EquipmentConstant{
			Ecid:  ecid,
			Name:  ec.Name,
			Units: ec.Units,
			Value: toJSON(ec.Value),
		})
	}
	return &ListECResponse{Constants: result}, nil
}

func (s *Server) SetEquipmentConstant(_ context.Context, req *SetECRequest) (*SetECResponse, error) {
	var val interface{}
	if err := json.Unmarshal([]byte(req.Value), &val); err != nil {
		val = req.Value // Use as string if not valid JSON
	}

	if err := s.handler.Variables().SetEC(req.Ecid, val); err != nil {
		return &SetECResponse{Success: false, Error: err.Error()}, nil
	}
	return &SetECResponse{Success: true}, nil
}

func (s *Server) ExecuteCommand(_ context.Context, req *ExecuteCommandRequest) (*ExecuteCommandResponse, error) {
	if req.Command == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}

	var params []gem.CommandParam
	for k, v := range req.Params {
		params = append(params, gem.CommandParam{Name: k, Value: v})
	}

	result := s.handler.Commands().Execute(context.Background(), req.Command, params)
	return &ExecuteCommandResponse{
		Command: req.Command,
		Status:  result.String(),
		Code:    int32(result),
	}, nil
}

func (s *Server) ListAlarms(_ context.Context, req *ListAlarmsRequest) (*ListAlarmsResponse, error) {
	var alarms []*gem.Alarm
	if req.ActiveOnly {
		alarms = s.handler.Alarms().ListActiveAlarms()
	} else {
		alarms = s.handler.Alarms().ListAlarms()
	}

	result := make([]*AlarmInfo, 0, len(alarms))
	for _, a := range alarms {
		result = append(result, &AlarmInfo{
			Alid:     a.ALID,
			Name:     a.Name,
			Text:     a.Text,
			State:    a.State.String(),
			Enabled:  a.Enabled,
			Severity: uint32(a.Severity),
		})
	}
	return &ListAlarmsResponse{Alarms: result}, nil
}

func (s *Server) StreamEvents(_ *StreamEventsRequest, stream SECSGEMService_StreamEventsServer) error {
	ch := make(chan eventMsg, 256)
	s.streamMu.Lock()
	s.streams[ch] = struct{}{}
	s.streamMu.Unlock()

	defer func() {
		s.streamMu.Lock()
		delete(s.streams, ch)
		s.streamMu.Unlock()
		close(ch)
	}()

	s.logger.Info("gRPC event stream started")

	for {
		select {
		case <-stream.Context().Done():
			s.logger.Info("gRPC event stream ended")
			return nil
		case msg := <-ch:
			dataJSON, _ := json.Marshal(msg.Data)
			if err := stream.Send(&EventNotification{
				Type:      msg.Type,
				Timestamp: msg.Timestamp,
				Data:      string(dataJSON),
			}); err != nil {
				return err
			}
		}
	}
}

func (s *Server) broadcastEvent(eventType string, data interface{}) {
	msg := eventMsg{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	for ch := range s.streams {
		select {
		case ch <- msg:
		default:
			// Stream client too slow, drop event
		}
	}
}

func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
