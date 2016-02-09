#!/bin/bash

program="121proxy"
gobin="`which go`"
repo="github.com/z0rr0/121proxy"
if [ -z "$GOPATH" ]; then
    echo "ERROR: set GOPATH env"
    exit 1
fi
if [ ! -x "$gobin" ]; then
    echo "ERROR: can't find 'go' binary"
    exit 2
fi

# prepare test config
cfgname="${GOPATH}/test.json"
if [[ ! -f ${GOPATH}/src/${repo}/config.json ]]; then
	echo "ERROR: not found file: ${GOPATH}/src/${repo}/config.json"
	exit 3
fi

cp -f ${GOPATH}/src/${repo}/config.json $cfgname
/bin/sed -i 's/\/\/.*$//g' $cfgname

cd ${GOPATH}/src/${repo}/proxy
go test -v -cover -coverprofile=coverage.out -trace trace.out || exit 1

# rm -f $cfgname

echo "all tests done, use next command to view profiling results:"
echo "go tool cover -html=<package_path>/coverage.out"
echo "go tool trace <package_path>/<package_name>.test <package_path>/trace.out"

# find ${GOPATH}/src -type f -name coverage.out -exec rm -f '{}' \;
# find ${GOPATH}/src -type f -name trace.out -exec rm -f '{}' \;
# find ${GOPATH}/src -type f -name "*.test" -exec rm -f '{}' \;

exit 0
