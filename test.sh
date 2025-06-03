#!/usr/bin/bash

cp tests/* .
go test -v
rm -f *_test.go

