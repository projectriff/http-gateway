# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: prepare test

pkg/serialization/riff-serialization.pb.go: riff-serialization.proto
	protoc -I . riff-serialization.proto --go_out=plugins=grpc:pkg/serialization

pkg/liiklus/LiiklusService.pb.go: LiiklusService.proto
	protoc -I . LiiklusService.proto --go_out=plugins=grpc:pkg/liiklus

.PHONY: test
test: fmt vet manifests ## Run tests
	go test ./... -coverprofile cover.out

.PHONY: compile
compile: prepare ko pkg/serialization/riff-serialization.pb.go pkg/liiklus/LiiklusService.pb.go ## Compile target binaries
	$(KO) resolve -L -f config/ > /dev/null

.PHONY: prepare
prepare: fmt vet manifests ## Create all generated and scaffolded files
	kustomize build config/http-gateway/default > config/riff-http-gateway.yaml

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests:
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=http-gateway-role \
		paths="./cmd/...;./pkg/..." \
		output:rbac:artifacts:config=./config/http-gateway/rbac

# Run go fmt against code
.PHONY: fmt
fmt: goimports
	$(GOIMPORTS) --local github.com/projectriff/system -w pkg/ cmd/

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# find or download controller-gen, download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	# avoid go.* mutations from go get
	cp go.mod go.mod~ && cp go.sum go.sum~
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0
	mv go.mod~ go.mod && mv go.sum~ go.sum
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download goimports, download goimports if necessary
goimports:
ifeq (, $(shell which goimports))
	# avoid go.* mutations from go get
	cp go.mod go.mod~ && cp go.sum go.sum~
	go get golang.org/x/tools/cmd/goimports@release-branch.go1.13
	mv go.mod~ go.mod && mv go.sum~ go.sum
GOIMPORTS=$(GOBIN)/goimports
else
GOIMPORTS=$(shell which goimports)
endif

# find or download ko, download ko if necessary
ko:
ifeq (, $(shell which ko))
	GO111MODULE=off go get github.com/google/ko/cmd/ko
KO=$(GOBIN)/ko
else
KO=$(shell which ko)
endif

# Absolutely awesome: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help: ## Print help for each make target
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
