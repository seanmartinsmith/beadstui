# bv Makefile
#
# Build with SQLite FTS5 (full-text search) support enabled

.PHONY: build install clean test

# Enable FTS5 for full-text search in SQLite exports
export CGO_CFLAGS := -DSQLITE_ENABLE_FTS5

build:
	go build -o bv ./cmd/bv

install:
	go install ./cmd/bv

clean:
	rm -f bv
	go clean

test:
	go test ./...
