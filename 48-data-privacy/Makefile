.PHONY: build test race fuzz vet fmt run clean

build:
	go build -o vault ./cmd/vault

test:
	go vet ./...
	go test ./...

race:
	go test -race ./...

fuzz:
	go test -run=NONE -fuzz=FuzzTokenizeRoundTrip -fuzztime=30s ./internal/vault

vet:
	go vet ./...

fmt:
	gofmt -w .

run: build
	./vault serve

clean:
	rm -f vault vault.exe vault.db
