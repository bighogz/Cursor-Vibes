.PHONY: build go-build rust-build go-run go-run-op go-scan deps clean frontend frontend-dev test checksums

VERSION  := $(shell cat VERSION 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS  := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build: go-build rust-build frontend

go-build:
	go build -ldflags "$(LDFLAGS)" -o bin/api ./cmd/api
	go build -o bin/scan ./cmd/scan

test:
	go vet ./...
	go test ./... -count=1 -race

rust-build:
	@command -v cargo >/dev/null 2>&1 || { echo "Install Rust: https://rustup.rs"; exit 1; }
	CARGO_TARGET_DIR=rust-core/target cargo build --release --manifest-path rust-core/Cargo.toml
	@mkdir -p bin && cp rust-core/target/release/vibes-anomaly bin/ || true

frontend:
	cd frontend && npm install && npm run build

frontend-dev:
	cd frontend && npm install && npm run dev

go-run: go-build
	./bin/api

# Inject secrets from 1Password at runtime — no plaintext .env on disk
# Requires: brew install 1password-cli && op signin
go-run-op: go-build
	op run --env-file=.env.tpl -- ./bin/api

go-scan: go-build
	./bin/scan

deps:
	go mod download
	go mod tidy
	cd frontend && npm install

checksums:
	@cd bin && shasum -a 256 api vibes-anomaly 2>/dev/null | tee SHA256SUMS

clean:
	rm -rf bin/ rust-core/target/ frontend/dist/ frontend/node_modules/
