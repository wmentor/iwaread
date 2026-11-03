.PHONY: all test/unit

all: test/unit

test/unit:
	go test -race ./...
