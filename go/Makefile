SHELL := /bin/bash

all: run

update:
	@go get -u
	@go mod tidy

run:
	@go run *.go

test:
	@go test -v ./...

lines:
	@find . -name '*.go' | xargs wc -l
