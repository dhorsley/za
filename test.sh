#!/usr/bin/bash

cp tests/* .
go test
rm -f *_test.go

