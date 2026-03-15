package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/kronveil/kronveil/core/engine"
	"github.com/kronveil/kronveil/intelligence/incident"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// Config holds gRPC server configuration.
type Config struct {
	Port        int    `yaml:"port" json:"port"`
	TLSCertFile string `yaml:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile  string `yaml:"tls_key_file" json:"tls_key_file"`
	TLSCAFile   string `yaml:"tls_ca_file" json:"tls_ca_file"`
	MutualTLS   bool   `yaml:"mutual_tls" json:"mutual_tls"`
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

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(loggingUnaryInterceptor),
		grpc.StreamInterceptor(loggingStreamInterceptor),
	}

	// Configure TLS if cert/key provided.
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		tlsCreds, err := buildGRPCTLSCredentials(s.config.TLSCertFile, s.config.TLSKeyFile, s.config.TLSCAFile, s.config.MutualTLS)
		if err != nil {
			return fmt.Errorf("failed to configure gRPC TLS: %w", err)
		}
		opts = append(opts, grpc.Creds(tlsCreds))
		log.Printf("[grpc] TLS enabled (mTLS: %v)", s.config.MutualTLS)
	}

	s.grpcServer = grpc.NewServer(opts...)

	// Register Kronveil gRPC service.
	svc := &KronveilService{
		engine:    s.engine,
		responder: s.responder,
	}
	RegisterKronveilService(s.grpcServer, svc)

	// Register gRPC reflection for debugging with grpcurl.
	reflection.Register(s.grpcServer)

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

// buildGRPCTLSCredentials creates gRPC transport credentials with optional mTLS.
func buildGRPCTLSCredentials(certFile, keyFile, caFile string, mutualTLS bool) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS key pair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if mutualTLS && caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return credentials.NewTLS(tlsConfig), nil
}
