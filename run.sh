#!/bin/bash
set -e
go run process.go $1 /tmp/output.wav
afplay /tmp/output.wav
