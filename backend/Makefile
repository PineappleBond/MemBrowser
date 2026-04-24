.PHONY: build run test lint clean

build:
	go build -o bin/membrowser ./cmd/server/

run: build
	./bin/membrowser

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ data/
