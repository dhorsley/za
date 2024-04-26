#!/bin/bash

seconds=${1:-20}

go tool pprof -http localhost:8080 http://localhost:8008/debug/pprof/profile?seconds=${seconds}

