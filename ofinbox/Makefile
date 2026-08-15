.PHONY: build test tidy clean

build:
	go build -o bin/ofinbox .

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
