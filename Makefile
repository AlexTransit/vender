GOCMD=go
GOBUILD=$(GOCMD) build
GOVULN=govulncheck
BINARY_NAME=./build/vender
TARGET_ENV=CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc GOOS=linux GOARCH=arm GOARM=7
VERSION=$(shell git describe --always --dirty --tags)

.PHONY: audit build64 build
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
	@# -ldflags="-s -w" критически важен для экономии RAM при запуске на 512MB
	GOMAXPROCS=2 $(TARGET_ENV) $(GOBUILD) -trimpath \
	-ldflags="-s -w -X 'main.BuildVersion=$(VERSION)'" \
	-o $(BINARY_NAME) ./cmd/vender
	@echo "===> Done. Binary: $(BINARY_NAME), Size: $$(du -h $(BINARY_NAME) | cut -f1)"
