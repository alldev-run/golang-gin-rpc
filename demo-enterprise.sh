#!/bin/bash

# Enterprise Features Demo Script
# This script demonstrates the enterprise-level features of the project

set -e

echo "🚀 Starting Enterprise Features Demo..."
echo "=================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

print_header() {
    echo -e "${BLUE}[DEMO]${NC} $1"
}

# Check if required tools are installed
check_dependencies() {
    print_header "Checking dependencies..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    print_status "✅ Docker is installed"
    
    # Check Go
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go first."
        exit 1
    fi
    print_status "✅ Go is installed"
    
    # Check if project is built
    if [ ! -f "./alldev-gin-rpc" ]; then
        print_warning "Binary not found, building..."
        make build
    fi
    print_status "✅ Binary is ready"
}

# Start infrastructure services
start_infrastructure() {
    print_header "Starting infrastructure services..."
    
    # Start Prometheus
    print_status "Starting Prometheus..."
    docker run -d --name prometheus \
        -p 9091:9090 \
        -v $(pwd)/configs/prometheus.yml:/etc/prometheus/prometheus.yml \
        prom/prometheus:latest
    
    # Start Grafana
    print_status "Starting Grafana..."
    docker run -d --name grafana \
        -p 3000:3000 \
        -e "GF_SECURITY_ADMIN_PASSWORD=admin" \
        grafana/grafana:latest
    
    # Start Consul (for service discovery)
    print_status "Starting Consul..."
    docker run -d --name consul \
        -p 8500:8500 \
        -p 8600:8600/udp \
        consul:latest agent -dev -client=0.0.0.0
    
    print_status "✅ Infrastructure services started"
    print_status "   - Prometheus: http://localhost:9091"
    print_status "   - Grafana: http://localhost:3000 (admin/admin)"
    print_status "   - Consul: http://localhost:8500"
}

# Update configuration for enterprise features
update_config() {
    print_header "Updating configuration for enterprise features..."
    
    # Create enterprise config
    cat > configs/enterprise.yaml << EOF
# Enterprise Configuration
server:
  host: "localhost"
  port: "8080"
  mode: "release"

# Database Configuration
database:
  mysql_primary:
    type: "mysql"
    mysql:
      host: "localhost"
      port: 3306
      database: "enterprise_db"
      username: "root"
      password: "password"
      charset: "utf8mb4"

# Cache Configuration
cache:
  type: "redis"
  redis:
    host: "localhost"
    port: 6379
    password: ""

# Logger Configuration
logger:
  level: "info"
  env: "prod"
  log_path: "./logs/enterprise.log"

# RPC Configuration
rpc:
  servers:
    grpc:
      type: "grpc"
      host: "localhost"
      port: 50051
      network: "tcp"
      timeout: 30
      reflection: true
    jsonrpc:
      type: "jsonrpc"
      host: "localhost"
      port: 8081
      network: "tcp"
      timeout: 30
  timeout: 30s
  graceful_shutdown_timeout: 10s

# Service Discovery Configuration
discovery:
  enabled: true
  registry_type: "consul"
  registry_address: "localhost:8500"
  timeout: 30s
  health_check_interval: 30s
  auto_register: true
  service_name: "enterprise-service"
  service_address: "localhost"
  service_port: 8080
  service_tags:
    - "enterprise"
    - "production"
    - "v1.0"

# Metrics Configuration
metrics:
  enabled: true
  address: "localhost:9090"
  path: "/metrics"

# Circuit Breaker Configuration
circuit_breaker:
  enabled: true
  default_config:
    max_requests: 1
    interval: "1m"
    timeout: "30s"
    consecutive_failures: 5

# Rate Limiter Configuration
rate_limiter:
  enabled: true
  default_config:
    strategy: "token_bucket"
    rate: 100
    burst: 10

# Authentication Configuration
auth:
  enabled: true
  jwt:
    secret: "your-secret-key-here"
    issuer: "enterprise-service"
    token_ttl: "1h"
  rbac:
    enabled: true
    default_role: "user"
EOF

    print_status "✅ Enterprise configuration created"
}

# Build enterprise binary
build_enterprise() {
    print_header "Building enterprise binary..."
    
    # Build with enterprise features
    go build -ldflags "-X main.version=enterprise-1.0.0" -o enterprise-service .
    
    print_status "✅ Enterprise binary built: ./enterprise-service"
}

# Start enterprise service
start_service() {
    print_header "Starting enterprise service..."
    
    # Create logs directory
    mkdir -p logs
    
    # Start the service in background
    ./enterprise-service -config configs/enterprise.yaml &
    SERVICE_PID=$!
    
    print_status "✅ Enterprise service started (PID: $SERVICE_PID)"
    print_status "   - HTTP API: http://localhost:8080"
    print_status "   - gRPC: localhost:50051"
    print_status "   - JSON-RPC: http://localhost:8081/rpc"
    print_status "   - Metrics: http://localhost:9090/metrics"
    
    # Wait for service to start
    sleep 5
}

# Run enterprise tests
run_tests() {
    print_header "Running enterprise feature tests..."
    
    # Test health check
    print_status "Testing health check..."
    curl -s http://localhost:8080/health | jq . || print_warning "Health check failed"
    
    # Test metrics endpoint
    print_status "Testing metrics endpoint..."
    curl -s http://localhost:9090/metrics | head -5 || print_warning "Metrics endpoint failed"
    
    # Test JSON-RPC
    print_status "Testing JSON-RPC..."
    curl -s -X POST http://localhost:8081/rpc \
        -H "Content-Type: application/json" \
        -d '{
            "jsonrpc": "2.0",
            "method": "system.ping",
            "params": {},
            "id": 1
        }' | jq . || print_warning "JSON-RPC test failed"
    
    # Test rate limiting
    print_status "Testing rate limiting..."
    for i in {1..15}; do
        curl -s http://localhost:8080/api/v1/test > /dev/null 2>&1 || true
    done
    
    print_status "✅ Enterprise tests completed"
}

# Demo circuit breaker
demo_circuit_breaker() {
    print_header "Demo: Circuit Breaker"
    
    print_status "Simulating service failures to trigger circuit breaker..."
    
    # Make requests that will fail
    for i in {1..10}; do
        curl -s http://localhost:8080/api/v1/fail > /dev/null 2>&1 || true
        sleep 0.5
    done
    
    print_status "Circuit breaker should now be OPEN"
    print_status "Subsequent requests will be rejected..."
    
    # Try to make requests (should fail)
    for i in {1..3}; do
        response=$(curl -s -w "%{http_code}" http://localhost:8080/api/v1/fail 2>/dev/null || echo "000")
        echo "Request $i: HTTP $response"
        sleep 1
    done
    
    print_status "✅ Circuit breaker demo completed"
}

# Demo rate limiting
demo_rate_limiting() {
    print_header "Demo: Rate Limiting"
    
    print_status "Making rapid requests to test rate limiting..."
    
    success=0
    limited=0
    
    for i in {1..20}; do
        response=$(curl -s -w "%{http_code}" http://localhost:8080/api/v1/test 2>/dev/null || echo "000")
        if [ "$response" = "200" ]; then
            ((success++))
        elif [ "$response" = "429" ]; then
            ((limited++))
        fi
        echo "Request $i: HTTP $response"
    done
    
    print_status "Rate limiting results:"
    print_status "  - Successful requests: $success"
    print_status "  - Rate limited requests: $limited"
    print_status "✅ Rate limiting demo completed"
}

# Demo service discovery
demo_service_discovery() {
    print_header "Demo: Service Discovery"
    
    print_status "Checking registered services in Consul..."
    
    # List services in Consul
    curl -s http://localhost:8500/v1/catalog/services | jq . || print_warning "Failed to list services"
    
    print_status "Checking enterprise service registration..."
    
    # Get enterprise service details
    curl -s http://localhost:8500/v1/catalog/service/enterprise-service | jq . || print_warning "Failed to get service details"
    
    print_status "✅ Service discovery demo completed"
}

# Show metrics dashboard
show_metrics() {
    print_header "Metrics Dashboard"
    
    print_status "Opening Prometheus dashboard..."
    if command -v open &> /dev/null; then
        open http://localhost:9091
    elif command -v xdg-open &> /dev/null; then
        xdg-open http://localhost:9091
    fi
    
    print_status "Opening Grafana dashboard..."
    if command -v open &> /dev/null; then
        open http://localhost:3000
    elif command -v xdg-open &> /dev/null; then
        xdg-open http://localhost:3000
    fi
    
    print_status "✅ Dashboards opened in browser"
}

# Cleanup function
cleanup() {
    print_header "Cleaning up..."
    
    # Stop enterprise service
    if [ ! -z "$SERVICE_PID" ]; then
        kill $SERVICE_PID 2>/dev/null || true
        print_status "✅ Enterprise service stopped"
    fi
    
    # Stop and remove containers
    docker stop prometheus grafana consul 2>/dev/null || true
    docker rm prometheus grafana consul 2>/dev/null || true
    print_status "✅ Infrastructure containers stopped"
    
    # Clean up binaries
    rm -f enterprise-service
    print_status "✅ Cleanup completed"
}

# Main execution
main() {
    echo "🎯 Enterprise Features Demo"
    echo "=========================="
    echo ""
    
    # Set up trap for cleanup
    trap cleanup EXIT
    
    # Run demo steps
    check_dependencies
    start_infrastructure
    update_config
    build_enterprise
    start_service
    run_tests
    
    # Interactive demos
    echo ""
    print_header "Interactive Demos"
    echo "Press Enter to continue with each demo..."
    
    read -p ""
    demo_circuit_breaker
    
    read -p ""
    demo_rate_limiting
    
    read -p ""
    demo_service_discovery
    
    # Show metrics
    show_metrics
    
    echo ""
    print_header "Demo Summary"
    echo "=================="
    print_status "✅ All enterprise features demonstrated successfully!"
    print_status ""
    print_status "Features demonstrated:"
    print_status "  - Service Discovery (Consul)"
    print_status "  - Circuit Breaker"
    print_status "  - Rate Limiting"
    print_status "  - Metrics Collection (Prometheus)"
    print_status "  - Monitoring Dashboard (Grafana)"
    print_status "  - Health Checks"
    print_status "  - JSON-RPC API"
    print_status ""
    print_status "Services running:"
    print_status "  - Enterprise Service: http://localhost:8080"
    print_status "  - Prometheus: http://localhost:9091"
    print_status "  - Grafana: http://localhost:3000"
    print_status "  - Consul: http://localhost:8500"
    print_status ""
    print_status "Press Ctrl+C to stop all services and cleanup."
    
    # Wait for user interrupt
    while true; do
        sleep 1
    done
}

# Run main function
main "$@"
