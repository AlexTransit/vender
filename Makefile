GOCMD=go
GOBUILD=$(GOCMD) build
GOVULN=govulncheck
BINARY_NAME=./build/vender
TARGET_ENV=CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc GOOS=linux GOARCH=arm GOARM=7
VERSION=$(shell git describe --always --dirty --tags)
UNAME_M := $(shell uname -m)

ifneq ($(filter aarch64 arm64 x86_64,$(UNAME_M)),)
    DEFAULT_TARGET := build64
else
    DEFAULT_TARGET := build
endif

.PHONY: all audit build64 build

all: $(DEFAULT_TARGET)
# .PHONY: audit build64 build
audit:
	@echo "===> Scanning for vulnerabilities..."
	@# Проверяем, установлен ли govulncheck, если нет — устанавливаем
	@command -v $(GOVULN) >/dev/null 2>&1 || { \
		echo "Installing govulncheck..."; \
		$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest; \
	}
	@# Запуск самой проверки
	@$(GOVULN) ./... && echo "===> [OK] No vulnerabilities found."
build64:
	@echo "===> Building for Native (ARM64)..."
	CGO_ENABLED=1 $(GOBUILD) "-ldflags=-X 'main.BuildVersion=$(VERSION)'" -o $(BINARY_NAME) ./cmd/vender

build:
	@echo "===> Building for ARM32 (Target: 512MB RAM)..."
	@mkdir -p ./go-tmp
	@# -ldflags="-s -w" критически важен для экономии RAM при запуске на 512MB
	GOMAXPROCS=2 GOTMPDIR=$(shell pwd)/go-tmp $(TARGET_ENV) $(GOBUILD) -trimpath \
	-ldflags="-s -w -X 'main.BuildVersion=$(VERSION)'" \
	-o $(BINARY_NAME) ./cmd/vender
	@rm -rf ./go-tmp
	@echo "===> Done. Binary: $(BINARY_NAME), Size: $$(du -h $(BINARY_NAME) | cut -f1)"
