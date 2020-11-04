#!/bin/bash
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w"
