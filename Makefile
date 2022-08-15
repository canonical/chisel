.PHONY: all
all:
	go generate internal/deb/version.go
	go build ./cmd/chisel
