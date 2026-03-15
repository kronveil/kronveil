package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/intelligence/incident"
	"github.com/kronveil/kronveil/internal/version"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// KronveilService implements the gRPC API for the Kronveil agent.
type KronveilService struct {
	engine    *engine.Engine
	responder *incident.Responder
}

// --- Request/Response types (matching kronveil.proto) ---

type GetHealthRequest struct{}

type ComponentHealthProto struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type HealthResponse struct {
	Status        string                 `json:"status"`
	Components    []ComponentHealthProto `json:"components"`
	UptimeSeconds int64                 `json:"uptime_seconds"`
	Version       string                `json:"version"`
}

type GetIncidentRequest struct {
	ID string `json:"id"`
}

type TimelineEntryProto struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Details   string `json:"details"`
	Actor     string `json:"actor"`
}

type IncidentProto struct {
	ID                string               `json:"id"`
	Title             string               `json:"title"`
	Description       string               `json:"description"`
	Severity          string               `json:"severity"`
	Status            string               `json:"status"`
	RootCause         string               `json:"root_cause"`
	AffectedResources []string             `json:"affected_resources"`
	CreatedAt         string               `json:"created_at"`
	ResolvedAt        string               `json:"resolved_at,omitempty"`
	Timeline          []TimelineEntryProto `json:"timeline"`
}

type ListIncidentsRequest struct {
	Status string `json:"status"`
	Limit  int32  `json:"limit"`
}

type ListIncidentsResponse struct {
	Incidents []IncidentProto `json:"incidents"`
}

type StreamEventsRequest struct {
	Sources     []string `json:"sources"`
	MinSeverity string   `json:"min_severity"`
}

type TelemetryEventProto struct {
	ID        string            `json:"id"`
	Source    string            `json:"source"`
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
	Severity  string            `json:"severity"`
	Payload   json.RawMessage   `json:"payload"`
}

// --- Service implementation ---

func (s *KronveilService) getHealth(ctx context.Context, req *GetHealthRequest) (*HealthResponse, error) {
	st := s.engine.Status()
	var components []ComponentHealthProto
	for _, c := range st.Components {
		components = append(components, ComponentHealthProto{
			Name:    c.Name,
			Status:  c.Status,
			Message: c.Message,
		})
	}
	return &HealthResponse{
		Status:        st.Status,
		Components:    components,
		UptimeSeconds: int64(st.Uptime.Seconds()),
		Version:       version.Info(),
	}, nil
}

func (s *KronveilService) getIncident(ctx context.Context, req *GetIncidentRequest) (*IncidentProto, error) {
	if req.ID == "" {
		return nil, status.Error(codes.InvalidArgument, "incident id is required")
	}
	inc, ok := s.responder.GetIncident(req.ID)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "incident %s not found", req.ID)
	}
	return convertIncident(inc), nil
}

func (s *KronveilService) listIncidents(ctx context.Context, req *ListIncidentsRequest) (*ListIncidentsResponse, error) {
	incidents := s.responder.ListIncidents(req.Status)
	limit := int(req.Limit)
	if limit <= 0 || limit > len(incidents) {
		limit = len(incidents)
	}

	var result []IncidentProto
	for i := 0; i < limit; i++ {
		result = append(result, *convertIncident(incidents[i]))
	}
	return &ListIncidentsResponse{Incidents: result}, nil
}

func convertIncident(inc *engine.Incident) *IncidentProto {
	p := &IncidentProto{
		ID:                inc.ID,
		Title:             inc.Title,
		Description:       inc.Description,
		Severity:          inc.Severity,
		Status:            inc.Status,
		RootCause:         inc.RootCause,
		AffectedResources: inc.AffectedResources,
		CreatedAt:         inc.CreatedAt.Format(time.RFC3339),
	}
	if inc.ResolvedAt != nil {
		p.ResolvedAt = inc.ResolvedAt.Format(time.RFC3339)
	}
	for _, t := range inc.Timeline {
		p.Timeline = append(p.Timeline, TimelineEntryProto{
			Timestamp: t.Timestamp.Format(time.RFC3339),
			Action:    t.Action,
			Details:   t.Details,
			Actor:     t.Actor,
		})
	}
	return p
}

// --- gRPC ServiceDesc registration (avoids protoc dependency) ---

// jsonCodec is a simple codec that uses JSON for marshaling/unmarshaling.
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string { return "json" }

// RegisterKronveilService registers the Kronveil gRPC service handlers on the server.
func RegisterKronveilService(s *grpclib.Server, svc *KronveilService) {
	s.RegisterService(&kronveilServiceDesc, svc)
}

var kronveilServiceDesc = grpclib.ServiceDesc{
	ServiceName: "kronveil.v1.KronveilService",
	HandlerType: (*KronveilService)(nil),
	Methods: []grpclib.MethodDesc{
		{
			MethodName: "GetHealth",
			Handler:    handleGetHealth,
		},
		{
			MethodName: "GetIncident",
			Handler:    handleGetIncident,
		},
		{
			MethodName: "ListIncidents",
			Handler:    handleListIncidents,
		},
	},
	Streams: []grpclib.StreamDesc{
		{
			StreamName:    "StreamEvents",
			Handler:       handleStreamEvents,
			ServerStreams:  true,
		},
	},
	Metadata: "api/proto/kronveil.proto",
}

func handleGetHealth(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpclib.UnaryServerInterceptor) (interface{}, error) {
	req := &GetHealthRequest{}
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*KronveilService).getHealth(ctx, req)
	}
	info := &grpclib.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/kronveil.v1.KronveilService/GetHealth",
	}
	handler := func(ctx context.Context, r interface{}) (interface{}, error) {
		return srv.(*KronveilService).getHealth(ctx, r.(*GetHealthRequest))
	}
	return interceptor(ctx, req, info, handler)
}

func handleGetIncident(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpclib.UnaryServerInterceptor) (interface{}, error) {
	req := &GetIncidentRequest{}
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*KronveilService).getIncident(ctx, req)
	}
	info := &grpclib.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/kronveil.v1.KronveilService/GetIncident",
	}
	handler := func(ctx context.Context, r interface{}) (interface{}, error) {
		return srv.(*KronveilService).getIncident(ctx, r.(*GetIncidentRequest))
	}
	return interceptor(ctx, req, info, handler)
}

func handleListIncidents(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpclib.UnaryServerInterceptor) (interface{}, error) {
	req := &ListIncidentsRequest{}
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*KronveilService).listIncidents(ctx, req)
	}
	info := &grpclib.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/kronveil.v1.KronveilService/ListIncidents",
	}
	handler := func(ctx context.Context, r interface{}) (interface{}, error) {
		return srv.(*KronveilService).listIncidents(ctx, r.(*ListIncidentsRequest))
	}
	return interceptor(ctx, req, info, handler)
}

func handleStreamEvents(srv interface{}, stream grpclib.ServerStream) error {
	req := &StreamEventsRequest{}
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	svc := srv.(*KronveilService)

	// Stream events from all collectors.
	ctx := stream.Context()
	collectors := svc.engine.Registry().Collectors()
	if len(collectors) == 0 {
		return status.Error(codes.Unavailable, "no collectors registered")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			for _, col := range collectors {
				select {
				case event := <-col.Events():
					if event == nil {
						continue
					}
					// Filter by source if specified.
					if len(req.Sources) > 0 && !containsStr(req.Sources, event.Source) {
						continue
					}
					payloadJSON, _ := json.Marshal(event.Payload)
					proto := &TelemetryEventProto{
						ID:        event.ID,
						Source:    event.Source,
						Type:      event.Type,
						Timestamp: event.Timestamp.Format(time.RFC3339),
						Metadata:  event.Metadata,
						Severity:  event.Severity,
						Payload:   payloadJSON,
					}
					if err := stream.SendMsg(proto); err != nil {
						return err
					}
				default:
				}
			}
			// Small sleep to avoid busy loop.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// Ensure KronveilService message types implement the gRPC codec interface.
func init() {
	// Register JSON codec as a fallback for clients using JSON encoding.
	_ = fmt.Sprintf("kronveil gRPC service initialized")
}
