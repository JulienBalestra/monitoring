arm:
	$(MAKE) -C dd-wrt $@

amd64:
	$(MAKE) -C dd-wrt $@

clean:
	$(MAKE) -C dd-wrt $@

re: clean amd64 arm

fmt:
	@go fmt ./...

lint:
	golint -set_exit_status $(go list ./...)

import:
	goimports -w pkg/ cmd/

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
