#!/bin/bash

benchfile="./benchmarks_test.go"

if ! [ -e $benchfile ]; then
    echo "--- Generating query benchmarks file ---"
    go run ./benchgen/main.go && goimports -w $benchfile
else
    echo "--- Benchmarks already generated ---"
fi
exit 0
