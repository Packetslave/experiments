package echo

import (
	"context"

	echov1 "github.com/packetslave/experiments/grpc/gen/echo/v1"
)

type Server struct {
	echov1.UnimplementedEchoServiceServer
}

func (s *Server) Echo(_ context.Context, req *echov1.EchoRequest) (*echov1.EchoResponse, error) {
	return &echov1.EchoResponse{Message: req.Message}, nil
}
