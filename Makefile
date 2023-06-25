build:
	mkdir -p bin
	go build -o bin/forgetclose ./cmd/main.go

build-debug:
	mkdir -p bin
	go build -gcflags="all=-N -l" -o bin/forgetclose ./cmd/main.go

test:
	go test -v ./pkg/analyzer