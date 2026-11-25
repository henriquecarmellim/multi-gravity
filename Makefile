VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test install dist

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o mgy .

test:
	go vet ./...
	go test ./...

install:
	./install.sh

# binários para todos os sistemas, nos nomes que o install.sh e o install.ps1 esperam
dist:
	mkdir -p dist
	for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
	  os=$${t%/*}; arch=$${t#*/}; mkdir -p dist/$${os}_$${arch}; \
	  GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$${os}_$${arch}/mgy . && \
	  tar -czf dist/mgy_$${os}_$${arch}.tar.gz -C dist/$${os}_$${arch} mgy; \
	done
	for arch in amd64 arm64; do \
	  mkdir -p dist/windows_$$arch; \
	  GOOS=windows GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/windows_$$arch/mgy.exe . && \
	  (cd dist/windows_$$arch && zip -q ../mgy_windows_$$arch.zip mgy.exe); \
	done
	ls -l dist
