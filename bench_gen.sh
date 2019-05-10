#!/bin/bash

benchfile="pkg/query/benchmarks_test.go"

if ! [ -e $benchfile ]; then
    echo "--- Generating query benchmarks file ---"
    go run pkg/query/benchgen/main.go && goimports -w $benchfile
fi
