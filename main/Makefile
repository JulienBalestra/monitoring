TARGET=monitoring
COMMIT=$(shell git rev-parse HEAD)
VERSION=$(shell git describe --exact-match --tags $(git log -n1 --pretty='%h'))

VERSION_FLAGS=-ldflags '-s -w \
    -X github.com/JulienBalestra/monitoring/cmd/version.Version=$(VERSION) \
    -X github.com/JulienBalestra/monitoring/cmd/version.Commit=$(COMMIT)'

arm:
	GOARCH=$@ GOARM=$(GOARM) go build -i -o $(TARGET)-$@ $(VERSION_FLAGS) .

arm64:
	GOARCH=$@ go build -i -o $(TARGET)-$@ $(VERSION_FLAGS) .

amd64:
	go build -o $(TARGET)-$@ $(VERSION_FLAGS) .

clean:
	$(RM) $(TARGET)-amd64 $(TARGET)-arm $(TARGET)-arm64

re: clean amd64 arm arm64
