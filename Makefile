.PHONY: build go-build go-run go-scan deps

build: go-build

go-build:
	go build -o bin/api ./cmd/api
	go build -o bin/scan ./cmd/scan

go-run: go-build
	./bin/api

go-scan: go-build
	./bin/scan

deps:
	go mod download
	go mod tidy
