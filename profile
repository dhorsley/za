#!/bin/bash

seconds=${1:-20}

go tool pprof -http localhost:6000 http://localhost:6060/debug/pprof/profile?seconds=${seconds}

