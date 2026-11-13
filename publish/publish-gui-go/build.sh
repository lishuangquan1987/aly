#!/bin/sh
set -eu

export GO111MODULE=on
export CGO_ENABLED=0

go version
go list std >/dev/null

go fmt ./...
go vet ./...
go test ./...

mkdir -p build/windows-amd64 build/linux-amd64 build/linux-arm64
GOOS=windows GOARCH=amd64 go build -trimpath -o build/windows-amd64/AlyPublish.exe .
GOOS=linux   GOARCH=amd64 go build -trimpath -o build/linux-amd64/AlyPublish .
GOOS=linux   GOARCH=arm64 go build -trimpath -o build/linux-arm64/AlyPublish .
ls -l build/windows-amd64 build/linux-amd64 build/linux-arm64