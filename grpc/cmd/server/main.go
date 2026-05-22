package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	echov1 "github.com/packetslave/experiments/grpc/gen/echo/v1"
	echoserver "github.com/packetslave/experiments/grpc/internal/echo"
)

func main() {
	addr := flag.String("addr", ":50051", "listen address")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	echov1.RegisterEchoServiceServer(s, &echoserver.Server{})

	fmt.Printf("server listening on %s\n", *addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
