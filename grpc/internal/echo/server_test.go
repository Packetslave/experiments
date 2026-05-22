package echo_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	echov1 "github.com/packetslave/experiments/grpc/gen/echo/v1"
	echoserver "github.com/packetslave/experiments/grpc/internal/echo"
)

func TestEchoIntegration(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	echov1.RegisterEchoServiceServer(s, &echoserver.Server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			// Serve returns a non-nil error when Stop is called; ignore it.
			_ = err
		}
	}()
	t.Cleanup(s.Stop)

	addr := lis.Addr().String()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	client := echov1.NewEchoServiceClient(conn)

	tests := []struct {
		name string
		msg  string
	}{
		{"simple message", "hello"},
		{"empty message", ""},
		{"unicode", "distributed 🔁 systems"},
		{"long message", string(make([]byte, 4096))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := client.Echo(ctx, &echov1.EchoRequest{Message: tc.msg})
			if err != nil {
				t.Fatalf("Echo() error = %v", err)
			}
			if resp.Message != tc.msg {
				t.Errorf("Echo() = %q, want %q", resp.Message, tc.msg)
			}
		})
	}
}
