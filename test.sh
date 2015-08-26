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
cfgname="${GOPATH}/test.conf"
cp -f ${GOPATH}/src/${repo}/config.conf $cfgname
/bin/sed -i 's/\/\/.*$//g' $cfgname

