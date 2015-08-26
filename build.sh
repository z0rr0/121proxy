#!/bin/bash

# Build script
# -v - verbose mode
# -f - clean before new build

VERBOSE=""
FORCE=""

program="121proxy"
gobin="`which go`"
gitbin="`which git`"
buildrep="github.com/z0rr0/121proxy/main"
repo="github.com/z0rr0/121proxy"

if [ -z "$GOPATH" ]; then
    echo "ERROR: set $GOPATH env"
    exit 1
fi
if [ ! -x "$gobin" ]; then
    echo "ERROR: can't find 'go' binary"
    exit 2
fi
if [ ! -x "$gitbin" ]; then
    echo "ERROR: can't find 'git' binary"
    exit 3
fi

function debug() {
    if [ -n $VERBOSE ]; then
        echo $1
    fi
}

cd ${GOPATH}/src/${repo}
gittag="`$gitbin tag | sort --version-sort | tail -1`"
gitver="`$gitbin log --oneline | head -1 `"
build="`date --utc +\"%F_%T\"`UTC"
gitver="git:${gitver:0:7}"
if [[ -z "$gittag" ]]; then
    gittag="Na"
fi
vars="-X main.Version=$gittag -X main.Revision=$gitver -X main.BuildDate=$build"

options=""
while getopts ":fv" opt; do
    case $opt in
        v)
            options="$options -v"
            VERBOSE="verbose"
            echo "$program vars: $vars"
            ;;
        f)
            FORCE="force"
            ;;
        \?)
            echo "Invalid option: -$OPTARG" >&2
            ;;
    esac
done

if [[ -n "$FORCE" ]]; then
    rm -f ${GOPATH}/bin/main*
fi

# cd ${GOPATH}/src/${repo}
# $gobin test -v -cover -coverprofile=coverage.out || exit 1

debug "install $options -ldflags \"$vars\" $buildrep"
$gobin install $options -ldflags "$vars" $buildrep

exit 0