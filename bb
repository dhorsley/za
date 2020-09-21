#!/usr/bin/env bash
if [[ "$1" == "escape" ]]; then
    go build -gcflags "-m -m" za
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
