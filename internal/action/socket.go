package action

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Executor interface {
	Execute(context.Context, Request) (Response, error)
}

type Client struct {
	Socket           string
	MaxResponseBytes int64
}

func (c Client) Execute(ctx context.Context, request Request) (Response, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", c.Socket)
	if err != nil {
		return Response{}, fmt.Errorf("connect to phosphor-actiond: %w", err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return Response{}, fmt.Errorf("send action request: %w", err)
	}
	limit := c.MaxResponseBytes
	if limit <= 0 {
		limit = MaxResponseBytes
	}
	var response Response
	if err := json.NewDecoder(io.LimitReader(conn, limit)).Decode(&response); err != nil {
		return Response{}, fmt.Errorf("read action response: %w", err)
	}
	if response.ProtocolVersion != ProtocolVersion {
		return Response{}, fmt.Errorf("unsupported action response protocol version %q", response.ProtocolVersion)
	}
	if response.RequestID != request.RequestID || response.RuleID != request.RuleID {
		return Response{}, fmt.Errorf("action response correlation mismatch")
	}
	return response, nil
}

type Server struct {
	Config Config
	Runner Runner
}

func NewServer(cfg Config) *Server {
	return &Server{Config: cfg, Runner: Runner{Config: cfg}}
}

func (s *Server) Serve(ctx context.Context) error {
	if err := ValidateConfig(s.Config); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Config.Socket), 0o755); err != nil {
		return fmt.Errorf("create actiond socket directory: %w", err)
	}
	if err := removeStaleSocket(s.Config.Socket); err != nil {
		return err
	}
	listener, err := net.Listen("unix", s.Config.Socket)
	if err != nil {
		return fmt.Errorf("listen on actiond socket: %w", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(s.Config.Socket)
	}()
	if err := os.Chmod(s.Config.Socket, 0o660); err != nil {
		return fmt.Errorf("set actiond socket permissions: %w", err)
	}
	log.Printf("phosphor-actiond listening on unix://%s with %d rules", s.Config.Socket, len(s.Config.Rules))
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-serveCtx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept actiond connection: %w", err)
		}
		go s.handleConnection(serveCtx, conn)
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Duration(MaxTimeoutMS+5000) * time.Millisecond))
	var request Request
	decoder := json.NewDecoder(io.LimitReader(conn, s.Config.MaxRequestBytes))
	if err := decoder.Decode(&request); err != nil {
		_ = json.NewEncoder(conn).Encode(Response{ProtocolVersion: ProtocolVersion, Error: "decode action request: " + err.Error(), ExitCode: -1})
		return
	}
	response := s.Runner.Run(ctx, request)
	_ = json.NewEncoder(conn).Encode(response)
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect actiond socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to replace non-socket path %q", path)
	}
	if conn, err := net.DialTimeout("unix", path, 100*time.Millisecond); err == nil {
		_ = conn.Close()
		return fmt.Errorf("actiond socket %q is already in use", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove stale actiond socket: %w", err)
	}
	return nil
}
