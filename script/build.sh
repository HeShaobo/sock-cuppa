#!/bin/bash

cd "$( dirname "${BASH_SOURCE[0]}" )/.."

version=`grep 'const VERSION = ' ./cuppa.go | sed -e 's/.*= //' | sed -e 's/"//g'`
echo "creating cuppa binary version $version"

ROOT=`pwd`

bindir=$ROOT/bin
mkdir -p $bindir

build() {
    local name
    local GOOS
    local GOARCH

    export CGO_ENABLED=0

    prog=sock-cuppa
    # pkgname
    name=$prog-$3-$version
    echo "building $name"
    GOOS=$1 GOARCH=$2 go build -a || exit 1
    mkdir -p $bindir/$name
    mv $prog $bindir/$name/$name
    cp script/service.sh $bindir/$name/
    pushd $bindir
    tar -zcvf $name.tar.gz $name
    rm -rf $name
    popd
}

# only for linux
build linux amd64 linux64
build linux 386 linux32
