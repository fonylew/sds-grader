#!/bin/sh
mkdir -p ../bin
cd activity7
GOOS=darwin GOARCH=arm64 go build -o ../bin/activity7-macos-arm64
GOOS=darwin GOARCH=amd64 go build -o ../bin/activity7-macos-intel
GOOS=linux GOARCH=amd64 go build -o ../bin/activity7-linux
GOOS=windows GOARCH=amd64 go build -o ../bin/activity7-windows.exe
chmod +x ../bin/*
