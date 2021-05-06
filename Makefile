BINARY_NAME=imap-cleaner

all: linux macos windows

clean:
	@go clean
	@rm -rf dist

deps:
	@go mod tidy

linux:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/${BINARY_NAME}-linux-x86_64

macos:
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/${BINARY_NAME}-macos-x86_64

windows:
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/${BINARY_NAME}-windows-x86_64.exe
