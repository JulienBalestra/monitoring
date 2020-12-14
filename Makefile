arm:
	$(MAKE) -C main $@

arm64:
	$(MAKE) -C main $@

amd64:
	$(MAKE) -C main $@

clean:
	$(MAKE) -C main $@

re: clean amd64 arm arm64

fmt:
	@go fmt ./...

lint:
	golint -set_exit_status $(go list ./...)

import:
	goimports -w pkg/ cmd/ main/

ineffassign:
	ineffassign ./

test:
	@go test -v -race ./...

vet:
	@go vet -v ./...

.pristine:
	git ls-files --exclude-standard --modified --deleted --others | diff /dev/null -

verify-fmt: fmt .pristine

verify-import: import .pristine

generate:
	@go run pkg/mac/main/main.go
