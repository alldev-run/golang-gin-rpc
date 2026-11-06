#!/bin/bash

# Golang Gin RPC Application Startup Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}')
    print_status "Go version: $GO_VERSION"
}

# Check if config file exists
check_config() {
    if [ ! -f "./configs/config.yaml" ]; then
        if [ -f "./configs/config.example.yaml" ]; then
            print_status "Configuration file not found, creating from configs/config.example.yaml..."
            mkdir -p ./configs
            cp ./configs/config.example.yaml ./configs/config.yaml
            print_status "Default configuration file created"
        else
            print_error "configs/config.yaml and configs/config.example.yaml not found"
            exit 1
        fi
    fi
}

# Create necessary directories
create_directories() {
    print_status "Creating necessary directories..."
    mkdir -p logs
    mkdir -p temp
}

# Download dependencies
download_deps() {
    print_status "Downloading Go dependencies..."
    go mod download
    go mod tidy
}

# Build the application
build_app() {
    print_status "Building application..."
    go build -o ./bin/alldev-gin-rpc .
    
    if [ $? -eq 0 ]; then
        print_status "Build successful"
    else
        print_error "Build failed"
        exit 1
    fi
}

# Run the application
run_app() {
    print_status "Starting application..."
    print_status "Application will be available at: http://localhost:8080"
    print_status "Health check endpoint: http://localhost:8080/health"
    print_status "Press Ctrl+C to stop the application"
    
    ./bin/alldev-gin-rpc
}

# Main execution
main() {
    print_status "Starting Golang Gin RPC Application..."
    
    check_go
    check_config
    create_directories
    download_deps
    build_app
    run_app
}

# Handle script arguments
case "${1:-}" in
    "build")
        check_go
        create_directories
        download_deps
        build_app
        print_status "Application built successfully"
        ;;
    "run")
        if [ ! -f "./bin/alldev-gin-rpc" ]; then
            print_warning "Binary not found, building first..."
            build_app
        fi
        run_app
        ;;
    "clean")
        print_status "Cleaning up..."
        rm -rf ./bin
        rm -rf ./logs
        rm -rf ./temp
        go clean -cache
        print_status "Cleanup completed"
        ;;
    "deps")
        check_go
        download_deps
        print_status "Dependencies downloaded"
        ;;
    "help"|"-h"|"--help")
        echo "Golang Gin RPC Application Startup Script"
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  (no args)  Full startup process (default)"
        echo "  build      Build the application only"
        echo "  run        Run the application only"
        echo "  clean      Clean build artifacts and caches"
        echo "  deps       Download dependencies only"
        echo "  help       Show this help message"
        echo ""
        echo "Examples:"
        echo "  $0              # Full startup"
        echo "  $0 build        # Build only"
        echo "  $0 run          # Run only"
        ;;
    *)
        main
        ;;
esac
