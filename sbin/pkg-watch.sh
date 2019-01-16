#!/usr/bin/env bash

PKG=NSFWGuard
CMD=$@

trap "kill -TERM -1; exit 143" SIGTERM

cd $GOPATH/src/$PKG

while true; do
    go get -v ./...
    $CMD &
    inotifywait -e modify -e move -e create -e delete -e attrib -r "$GOPATH/src/$PKG"
    kill -TERM -1
done;
