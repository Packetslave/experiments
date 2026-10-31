.PHONY: build test tidy clean

build:
	go build -o bin/ofinbox .

install: build
	cp bin/ofinbox $(HOME)/bin/ofinbox
	chmod +x $(HOME)/bin/ofinbox

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
