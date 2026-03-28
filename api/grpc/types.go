// types.go contains hand-written message types matching secsgem.proto.
// These replace protoc-generated code to keep the build simple (no protoc dependency).
// The types are wire-compatible with the proto definitions for JSON marshaling.
package grpcapi

import (
	"context"

	"google.golang.org/grpc"
)

// --- Request/Response types ---

type GetStatusRequest struct{}
type GetStatusResponse struct {
	CommState      string `json:"comm_state"`
	ControlState   string `json:"control_state"`
	Communicating  bool   `json:"communicating"`
	Online         bool   `json:"online"`
	TransportState string `json:"transport_state"`
}

type ListSVRequest struct{}
type StatusVariable struct {
	Svid  uint32 `json:"svid"`
	Name  string `json:"name"`
	Units string `json:"units"`
	Value string `json:"value"`
}
type ListSVResponse struct {
	Variables []*StatusVariable `json:"variables"`
}

type GetSVRequest struct {
	Svid uint32 `json:"svid"`
}
type GetSVResponse struct {
	Variable *StatusVariable `json:"variable"`
}

type ListECRequest struct{}
type EquipmentConstant struct {
	Ecid  uint32 `json:"ecid"`
	Name  string `json:"name"`
	Units string `json:"units"`
	Value string `json:"value"`
}
type ListECResponse struct {
	Constants []*EquipmentConstant `json:"constants"`
}

type SetECRequest struct {
	Ecid  uint32 `json:"ecid"`
	Value string `json:"value"`
}
type SetECResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type ExecuteCommandRequest struct {
	Command string            `json:"command"`
	Params  map[string]string `json:"params"`
}
type ExecuteCommandResponse struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Code    int32  `json:"code"`
}

type ListAlarmsRequest struct {
	ActiveOnly bool `json:"active_only"`
}
type AlarmInfo struct {
	Alid     uint32 `json:"alid"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	State    string `json:"state"`
	Enabled  bool   `json:"enabled"`
	Severity uint32 `json:"severity"`
}
type ListAlarmsResponse struct {
	Alarms []*AlarmInfo `json:"alarms"`
}

type StreamEventsRequest struct{}
type EventNotification struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Data      string `json:"data"`
}

// --- gRPC Service Interface ---

// SECSGEMServiceServer is the server-side interface.
type SECSGEMServiceServer interface {
	GetStatus(context.Context, *GetStatusRequest) (*GetStatusResponse, error)
	ListStatusVariables(context.Context, *ListSVRequest) (*ListSVResponse, error)
	GetStatusVariable(context.Context, *GetSVRequest) (*GetSVResponse, error)
	ListEquipmentConstants(context.Context, *ListECRequest) (*ListECResponse, error)
	SetEquipmentConstant(context.Context, *SetECRequest) (*SetECResponse, error)
	ExecuteCommand(context.Context, *ExecuteCommandRequest) (*ExecuteCommandResponse, error)
	ListAlarms(context.Context, *ListAlarmsRequest) (*ListAlarmsResponse, error)
	StreamEvents(*StreamEventsRequest, SECSGEMService_StreamEventsServer) error
}

// SECSGEMService_StreamEventsServer is the server-side streaming interface.
type SECSGEMService_StreamEventsServer interface {
	Send(*EventNotification) error
	grpc.ServerStream
}

// RegisterSECSGEMServiceServer registers the service with a gRPC server.
// This is a simplified registration that uses gRPC's generic service descriptor.
func RegisterSECSGEMServiceServer(s *grpc.Server, srv SECSGEMServiceServer) {
	sd := grpc.ServiceDesc{
		ServiceName: "secsgem.v1.SECSGEMService",
		HandlerType: (*SECSGEMServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "GetStatus", Handler: makeUnaryHandler(srv.GetStatus)},
			{MethodName: "ListStatusVariables", Handler: makeUnaryHandler(srv.ListStatusVariables)},
			{MethodName: "GetStatusVariable", Handler: makeUnaryHandler(srv.GetStatusVariable)},
			{MethodName: "ListEquipmentConstants", Handler: makeUnaryHandler(srv.ListEquipmentConstants)},
			{MethodName: "SetEquipmentConstant", Handler: makeUnaryHandler(srv.SetEquipmentConstant)},
			{MethodName: "ExecuteCommand", Handler: makeUnaryHandler(srv.ExecuteCommand)},
			{MethodName: "ListAlarms", Handler: makeUnaryHandler(srv.ListAlarms)},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "StreamEvents",
				ServerStreams:  true,
				Handler: func(srv interface{}, stream grpc.ServerStream) error {
					return srv.(SECSGEMServiceServer).StreamEvents(&StreamEventsRequest{}, &streamEventsServer{stream})
				},
			},
		},
		Metadata: "secsgem.proto",
	}
	s.RegisterService(&sd, srv)
}

type streamEventsServer struct {
	grpc.ServerStream
}

func (s *streamEventsServer) Send(m *EventNotification) error {
	return s.ServerStream.SendMsg(m)
}

// makeUnaryHandler creates a gRPC unary handler from a typed function.
// Uses encoding/json for simplicity instead of protobuf wire format.
func makeUnaryHandler[Req any, Resp any](fn func(context.Context, *Req) (*Resp, error)) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		req := new(Req)
		if err := dec(req); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return fn(ctx, req)
		}
		info := &grpc.UnaryServerInfo{Server: srv}
		return interceptor(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return fn(ctx, req.(*Req))
		})
	}
}
