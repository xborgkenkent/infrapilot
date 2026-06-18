.PHONY: build run tidy test clean

BINARY := infrapilot

build:
	go build -o bin/$(BINARY) ./cmd/infrapilot

run: build
	./bin/$(BINARY)

tidy:
	go mod tidy

test:
	go test ./...

clean:
	rm -rf bin/
