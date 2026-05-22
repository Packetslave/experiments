.PHONY: all generate build server client tidy clean

PROTO_SRC := api/echo/v1/echo.proto

all: build

# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc on PATH
# Install: go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
#          go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
generate:
	protoc \
	    --proto_path=api \
	    --go_out=gen --go_opt=paths=source_relative \
	    --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
	    $(PROTO_SRC)

build: server client

server:
	go build -o bin/server ./cmd/server

client:
	go build -o bin/client ./cmd/client

tidy:
	go mod tidy

clean:
	rm -rf bin/
