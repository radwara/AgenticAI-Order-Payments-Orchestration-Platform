.PHONY: build test lint gen

# Pinned code generators (recorded here; installed into ./bin, which is gitignored).
BUF_VERSION := v1.50.0
PROTOC_GEN_GO_VERSION := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.5.1

build:
	go build ./...

test:
	go vet ./...
	go test -race -cover ./...

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
	golangci-lint run ./...

# Regenerate gen/pricingv1 from contracts/pricing; must leave no git diff.
gen:
	GOBIN=$(CURDIR)/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(CURDIR)/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION) generate ../../contracts/pricing --template buf.gen.yaml
