TARGET=monitoring
COMMIT=$(shell git rev-parse HEAD)
VERSION=$(shell git describe --exact-match --tags $(git log -n1 --pretty='%h'))

VERSION_FLAGS=-ldflags '-s -w \
    -X github.com/JulienBalestra/dry/pkg/version.Version=$(VERSION) \
    -X github.com/JulienBalestra/dry/pkg/version.Commit=$(COMMIT)'

build:
	go build -o bin/$(TARGET) $(VERSION_FLAGS) main/main.go

arm:
	GOOS=linux GOARCH=$@ GOARM=$(GOARM) go build -o bin/$(TARGET)-linux-$@ $(VERSION_FLAGS) main/main.go

arm64:
	GOOS=linux GOARCH=$@ go build -o bin/$(TARGET)-linux-$@ $(VERSION_FLAGS) main/main.go

amd64:
	GOOS=linux GOARCH=$@ go build -o bin/$(TARGET)-linux-$@ $(VERSION_FLAGS) main/main.go

clean: fmt lint import ineffassign test vet
	$(RM) -v bin/*

re: clean build arm arm64 amd64

fmt:
	@go fmt ./...

lint:
	golint -set_exit_status $(go list ./...)

import:
	goimports -w pkg/ main/ cmd/

test:
	@go test -v -race ./...

vet:
	@go vet -v ./...

.pristine:
	git ls-files --exclude-standard --modified --deleted --others | diff /dev/null -

verify-fmt: fmt .pristine

verify-import: import .pristine

generate:
	@go run pkg/macvendor/main/main.go
