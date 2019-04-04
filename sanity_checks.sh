#!/bin/bash

echo "--- Installing binaries ---"
if ! out=$( go install ./... ); then
    echo -e "FAILED:\n$out"
    exit 1
fi
echo -e "OK $out"

echo "--- Running Linter ---"
out=$( golint ./... | grep -Ev "(annoying|MixedCaps|ColIdxAttributeCount)" )
if [[ $out != "" ]]; then
    echo -e "FAILED:\n$out"
    exit 1
fi
echo -e "OK $out"

echo "--- Running Tests ---"
if ! out=$( go test ./... | grep -v "no test files" ); then
    echo -e "FAILED:\n$out"
    exit 1
fi
echo -e "OK\n$out"

exit 0
