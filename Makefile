BUILD_TIME ?= $(shell date +%Y-%m-%d-%H%M-%Z)
SHORT_SHA ?= $(shell git rev-parse --short HEAD)

daemon:
	CGO_ENABLED=0 go build -mod=readonly -o bin/spiderpool cmd/daemon/main.go

test:
	go test -v ./...

image:
	docker build . --file build/daemon/release.Dockerfile --tag daocloud.io/daocloud/spiderpool-ci:$(BUILD_TIME)-$(SHORT_SHA)

.PHONY: all daemon test
