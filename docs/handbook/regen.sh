#!/usr/bin/bash
version=$(cat ../../VERSION)
pandoc handbook_${version}.md -s --toc -o index.html
