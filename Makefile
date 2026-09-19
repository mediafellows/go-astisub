.PHONY: build test integration-test release
build:
	go build ./...
test:
	go test -race ./...
integration-test:
	@echo 'TODO: no separate integration suite; conversion regressions run with make test.'
release: build test
	@echo 'Validation complete; publishing and merging are explicit PR operations.'
