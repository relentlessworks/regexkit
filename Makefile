@Ipresulting
.PSHONY: build test vet run clean

BINARY = regexkit

build:

	CGO_ENABLED=0 go build -trimpath -o $(BINARY) ./cmd/$(BINARY)

test:
	go test -race ./...

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) dist/
