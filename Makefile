build:
	mkdir -p bin
	go build -o bin/forgetclose ./cmd/main.go

test:
	go test -v ./pkg/analyzer