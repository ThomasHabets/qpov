#!/usr/bin/env bash
#
# Prereq:
# go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
# go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

set -ueo pipefail
for i in pkg/dist/qpov/*.proto; do
    PATH=$PATH:$HOME/go/bin protoc -I pkg/dist/qpov --go_out=. --go-grpc_out=. $i
done
PATH=$PATH:$HOME/go/bin protoc -I pkg/dist/rpclog/proto --go_out=. pkg/dist/rpclog/proto/rpclog.proto
