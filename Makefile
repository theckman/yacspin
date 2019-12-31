# Copyright (c) 2019 Tim Heckman
# Use of this source code is governed by the Apache 2.0 License that can be
# found in the LICENSE file at the root of this repository.

test: vet lint staticcheck tests

prebuild:
	go get -v -u github.com/golang/dep/cmd/dep \
		golang.org/x/lint/golint \
		honnef.co/go/tools/cmd/staticcheck \
		golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow \
		golang.org/x/tools/cmd/cover

# this needs to be updated when a future version of 1.13.x includes:
# https://github.com/golang/go/issues/34053
#
# original line now removed:
# go vet -vettool=$(shell which shadow) ./...
vet:
	go vet ./...
	shadow ./...

lint:
	golint -set_exit_status

staticcheck:
	staticcheck ./...

tests:
	go test -race -cover -covermode atomic -coverprofile coverage.txt ./...
	go tool cover -func=coverage.txt

.PHONY: test prebuild vet lint staticcheck tests
