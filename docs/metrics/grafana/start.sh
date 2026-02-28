#!/bin/bash

set -e

echo "=========================================="
echo "Za Grafana + Prometheus Docker Setup"
echo "=========================================="
echo ""

# Check if docker and docker-compose are available
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed"
    exit 1
fi

# Check if prometheus config exists
if [ ! -f "etc-prometheus/prometheus.yml" ]; then
    echo "❌ Prometheus config not found at etc-prometheus/prometheus.yml"
    exit 1
fi

echo "✅ Dependencies found"
echo ""

# Check if Za is running
echo "Checking if Za metrics are accessible..."
if timeout 1 nc -z localhost 9091 2>/dev/null; then
    echo "✅ Za metrics server is reachable on port 9091"
else
    echo "⚠️  Za metrics server not found on port 9091"
    echo "   Make sure to start Za with:"
    echo "   export ZA_PROMETHEUS=9091"
    echo "   export ZA_PROMETHEUS_CIDR='0.0.0.0/0'"
    echo "   ./za"
    echo ""
fi

echo "Starting Docker containers..."
docker-compose up -d

# Wait for services to start
echo "Waiting for services to start..."
sleep 5

echo ""
echo "=========================================="
echo "✅ Setup Complete!"
echo "=========================================="
echo ""
echo "Grafana Dashboard: http://localhost:3000"
echo "  - Username: admin"
echo "  - Password: admin"
echo ""
echo "Prometheus Targets: http://localhost:9090/targets"
echo ""
echo "Dashboard: Za Application Metrics"
echo ""
echo "To view logs:"
echo "  docker logs za-grafana -f"
echo "  docker logs za-prometheus -f"
echo ""
echo "To stop containers:"
echo "  docker-compose down"
echo ""
