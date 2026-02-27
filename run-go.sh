#!/bin/sh
# Run the Go API server (port 8000)
cd "$(dirname "$0")"
go run ./cmd/api
