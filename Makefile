.PHONY: setup generate starter

setup:
	@git config core.hooksPath .githooks
	@echo "Git hooks path set to .githooks"

pre-commit: generate
	@git add .

generate:
	@go generate ./... && go mod tidy

starter: generate
	@rm -rf ./starter/__example && \
		mkdir -p ./starter/__example && \
		go run ./cmd/genx init --target-dir=./starter/__example --go-module=github.com/molon/genx/starter/__example
