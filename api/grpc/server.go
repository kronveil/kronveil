package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/intelligence/incident"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Config holds gRPC server configuration.
type Config struct {
	Port int `yaml:"port" json:"port"`
}

// Server is the gRPC API server for Kronveil.
type Server struct {
	config     Config
	engine     *engine.Engine
	responder  *incident.Responder
	grpcServer *grpc.Server
	lis        net.Listener
}

// NewServer creates a new gRPC server.
func NewServer(config Config, eng *engine.Engine, resp *incident.Responder) *Server {
	return &Server{
		config:    config,
		engine:    eng,
		responder: resp,
	}
}

// loggingUnaryInterceptor logs gRPC unary calls.
func loggingUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Printf("[grpc] %s %v %v", info.FullMethod, time.Since(start), err)
	return resp, err
}

// loggingStreamInterceptor logs gRPC stream calls.
func loggingStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	log.Printf("[grpc] stream %s %v %v", info.FullMethod, time.Since(start), err)
	return err
}

// Start begins serving the gRPC API.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.config.Port, err)
	}
	s.lis = lis

	s.grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(loggingUnaryInterceptor),
		grpc.StreamInterceptor(loggingStreamInterceptor),
	)

	// Register gRPC reflection for debugging with grpcurl.
	reflection.Register(s.grpcServer)

	// NOTE: Proto-generated service registration goes here once proto code is generated:
	// proto.RegisterKronveilServiceServer(s.grpcServer, s)

	go func() {
		log.Printf("[grpc] gRPC server listening on :%d", s.config.Port)
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Printf("[grpc] gRPC server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the gRPC server.
func (s *Server) Stop() error {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	return nil
}
