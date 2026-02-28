#!/bin/bash

set -e

echo "===================================="
echo "Za Grafana + Prometheus Docker Setup"
echo "===================================="
echo

# Check if docker and docker-compose are available
if ! command -v docker &> /dev/null; then
    echo "x  Docker is not installed"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "x  Docker Compose is not installed"
    exit 1
fi

# Check if prometheus config exists
promconf="etc-prometheus/prometheus.yml"
if [ ! -f "${promconf}.template" ]; then
    echo "x  Prometheus config not found at ${promconf}.template"
    exit 1
fi

echo "Replacing __TARGETIP__ in prometheus.yml"
TARGETIP="$(ip route | grep "default" | grep -Eo " src .*? " | awk '{print $2}')"
cp ${promconf}.template ${promconf}
sed -i "s/__TARGETIP__/${TARGETIP}/g" ${promconf}

echo "o  Dependencies found"
echo ""

# Check if Za is running
echo "Checking if Za metrics are accessible..."
if timeout 1 nc -z localhost 9091 2>/dev/null; then
    echo "o  Za metrics server is reachable on port 9091"
else
    echo "x  Za metrics server not found on port 9091"
    echo "   Make sure to start Za with:"
    echo "   export ZA_PROMETHEUS=9091"
    echo "   export ZA_PROMETHEUS_CIDR='0.0.0.0/0'"
    echo "   ./za <script_name>"
    echo
fi

echo "Starting Docker containers..."
sudo docker-compose up -d

# Wait for services to start
echo "Waiting for services to start..."
sleep 3

echo
echo "==============="
echo "Setup Complete!"
echo "==============="
echo
echo "Grafana Dashboard: http://localhost:3000"
echo "  - Username: admin"
echo "  - Password: admin"
echo
echo "Prometheus Targets: http://localhost:9090/targets"
echo
echo "Dashboard: Za Application Metrics"
echo
echo "To view logs:"
echo "  sudo docker logs za-grafana -f"
echo "  sudo docker logs za-prometheus -f"
echo
echo "To stop containers:"
echo "  sudo docker-compose down"
echo

