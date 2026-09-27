.PHONY: test test-race vet build install build-all test-windows-build

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

build:
	go build -o bin/learn ./cmd/learn

install:
	go install ./cmd/learn

build-all:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -o dist/learn-darwin-arm64 ./cmd/learn
	GOOS=darwin GOARCH=amd64 go build -o dist/learn-darwin-amd64 ./cmd/learn
	GOOS=linux GOARCH=arm64 go build -o dist/learn-linux-arm64 ./cmd/learn
	GOOS=linux GOARCH=amd64 go build -o dist/learn-linux-amd64 ./cmd/learn
	GOOS=windows GOARCH=amd64 go build -o dist/learn-windows-amd64.exe ./cmd/learn
	GOOS=windows GOARCH=arm64 go build -o dist/learn-windows-arm64.exe ./cmd/learn

test-windows-build:
	GOOS=windows GOARCH=amd64 go vet ./...
	GOOS=windows GOARCH=amd64 go test -count=1 -run XXX_NO_TEST -exec /usr/bin/true ./... >/dev/null
