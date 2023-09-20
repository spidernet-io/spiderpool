.PHONY: test
test:
	go test  $(shell go list ./... ./godeltaprof/compat/... ./godeltaprof/...)

.PHONY: go/mod
go/mod:
	GO111MODULE=on go mod download
	go work sync
	GO111MODULE=on go mod tidy
	cd godeltaprof/compat/ && GO111MODULE=on go mod download
	cd godeltaprof/compat/ && GO111MODULE=on go mod tidy
	cd godeltaprof/  && GO111MODULE=on go mod download
	cd godeltaprof/ && GO111MODULE=on go mod tidy
