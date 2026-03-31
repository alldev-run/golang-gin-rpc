#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT_DIR/proto"
GO_BIN_DIR="$(go env GOPATH)/bin"

if [[ ":$PATH:" != *":$GO_BIN_DIR:"* ]]; then
  export PATH="$GO_BIN_DIR:$PATH"
fi

if ! command -v protoc >/dev/null 2>&1; then
  echo "Error: protoc is not installed."
  echo "Install protoc first: https://grpc.io/docs/protoc-installation/"
  exit 1
fi

if ! command -v protoc-gen-go >/dev/null 2>&1; then
  echo "Error: protoc-gen-go is not installed."
  echo "Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
  exit 1
fi

if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo "Error: protoc-gen-go-grpc is not installed."
  echo "Install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
  exit 1
fi

mapfile -t PROTO_FILES < <(find "$PROTO_DIR" -type f -name "*.proto" | sort)

if [[ ${#PROTO_FILES[@]} -eq 0 ]]; then
  echo "No proto files found under $PROTO_DIR"
  exit 0
fi

echo "Generating protobuf code..."

for proto_file in "${PROTO_FILES[@]}"; do
  rel_path="${proto_file#$ROOT_DIR/}"
  echo "  - $rel_path"

  protoc \
    -I "$ROOT_DIR" \
    --go_out="$ROOT_DIR" \
    --go_opt=paths=source_relative \
    --go-grpc_out="$ROOT_DIR" \
    --go-grpc_opt=paths=source_relative \
    "$rel_path"
done

echo "Proto generation complete."
