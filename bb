#!/usr/bin/env bash
go build za
if [[ $? == 0 ]]; then
    sudo cp za /usr/bin/
    echo "built and copied."
    exit 0
fi
echo "build failed."
