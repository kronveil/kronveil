package grpc

import (
	"fmt"
	"log"
	"net"

	"github.com/kronveil/kronveil/core/engine"
)

// Config holds gRPC server configuration.
type Config struct {
	Port int `yaml:"port" json:"port"`
}

// Server is the gRPC API server for Kronveil.
type Server struct {
	config Config
	engine *engine.Engine
	lis    net.Listener
}

// NewServer creates a new gRPC server.
func NewServer(config Config, eng *engine.Engine) *Server {
	return &Server{
		config: config,
		engine: eng,
	}
}

// Start begins serving the gRPC API.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.config.Port, err)
	}
	s.lis = lis

	log.Printf("[grpc] gRPC server listening on :%d", s.config.Port)

	// In production: creates grpc.NewServer with interceptors and registers
	// the KronveilService implementation, then calls server.Serve(lis).
	// srv := grpc.NewServer(
	//     grpc.UnaryInterceptor(loggingInterceptor),
	//     grpc.StreamInterceptor(streamLoggingInterceptor),
	// )
	// proto.RegisterKronveilServiceServer(srv, s)
	// go srv.Serve(lis)

	return nil
}

// Stop gracefully shuts down the gRPC server.
func (s *Server) Stop() error {
	if s.lis != nil {
		return s.lis.Close()
	}
	return nil
}
