BINARY := pv3
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install clean release

# Build for current platform
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Install to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" .

# Install to /usr/local/bin
install-global: build
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)

# Cross-compile for all release targets
release: clean
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/pv3-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/pv3-darwin-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/pv3-linux-arm64 .
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/pv3-linux-amd64 .

clean:
	rm -rf $(BINARY) dist/
