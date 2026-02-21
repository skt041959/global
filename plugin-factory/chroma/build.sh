#!/bin/bash
set -e

# Ensure we are in the plugin directory
cd "$(dirname "$0")"

# Initialize module if not already (redundant if already done, but safe)
if [ ! -f go.mod ]; then
    go mod init chroma-plugin
fi

# Fetch dependencies
go get github.com/alecthomas/chroma/v2
go mod tidy

# Build the shared object
# We need to tell CGO where parser.h is.
export CGO_CFLAGS="-I../../libparser"

echo "Building chroma-parser.so..."
go build -buildmode=c-shared -o chroma-parser.so main.go

echo "Build complete: $(pwd)/chroma-parser.so"
