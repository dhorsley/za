#!/bin/bash

seconds=${1:-20}

go tool pprof -http 0.0.0.0:6060 http://localhost:8008/debug/pprof/profile?seconds=${seconds}

