#!/usr/bin/env bash

if [[ "$1" == "alpine" ]]; then
    CGO_ENABLED=0 GOOS=linux GOARC=amd64 go build -tags 'usergo netgo' -installsuffix netgo
    echo "build alpine compatible starter build."
    echo "copy this to your executable location and run ./build alpine"
    exit 0
fi

if [[ "$1" == "escape" ]]; then
    go build -ldflags="-extldflags=-static" -gcflags "-m -m" za
    exit 0
else
    go build za
fi
if [[ $? == 0 ]]; then
    sudo cp za /usr/bin/
    echo "built and copied."
    exit 0
fi
echo "build failed."
