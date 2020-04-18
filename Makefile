TARGET=metrics
REVISION=$(shell git rev-parse HEAD)
VERSION=

VERSION_FLAGS=-ldflags '-s -w \
    -X github.com/JulienBalestra/metrics/cmd/version.Version=$(VERSION) \
    -X github.com/JulienBalestra/metrics/cmd/version.Revision=$(REVISION)'

arm:
	GOARCH=arm GOARM=5 go build -o $(TARGET)-arm $(VERSION_FLAGS) .

amd64:
	go build -o $(TARGET)-amd64 $(VERSION_FLAGS) .

clean:
	$(RM) $(TARGET)-amd64 $(TARGET)-arm

re: clean amd64 arm

fmt:
	@go fmt ./...

test:
	@go test -v ./...
