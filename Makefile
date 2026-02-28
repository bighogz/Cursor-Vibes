.PHONY: build go-build rust-build go-run go-scan deps

build: go-build rust-build

go-build:
	go build -o bin/api ./cmd/api
	go build -o bin/scan ./cmd/scan

rust-build:
	@command -v cargo >/dev/null 2>&1 || { echo "Install Rust: https://rustup.rs"; exit 1; }
	CARGO_TARGET_DIR=rust-core/target cargo build --release --manifest-path rust-core/Cargo.toml
	@mkdir -p bin && cp rust-core/target/release/vibes-anomaly bin/ || true

go-run: go-build
	./bin/api

go-scan: go-build
	./bin/scan

deps:
	go mod download
	go mod tidy
