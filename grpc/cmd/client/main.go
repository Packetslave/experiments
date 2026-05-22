package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	echov1 "github.com/packetslave/experiments/grpc/gen/echo/v1"
)

func main() {
	addr := flag.String("addr", "localhost:50051", "server address")
	msg := flag.String("msg", "hello", "message to echo")
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	client := echov1.NewEchoServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Echo(ctx, &echov1.EchoRequest{Message: *msg})
	if err != nil {
		log.Fatalf("echo failed: %v", err)
	}
	fmt.Println(resp.Message)
}
