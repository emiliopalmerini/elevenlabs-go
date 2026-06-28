.PHONY: fmt test vet check

fmt:
	gofmt -w *.go

test:
	go test ./...

vet:
	go vet ./...

check: fmt vet test
