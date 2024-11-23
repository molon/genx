.PHONY: setup generate starter

setup:
	@git config core.hooksPath .githooks
	@echo "Git hooks path set to .githooks"

pre-commit: generate
	@git add .

generate:
	@go generate ./... && go mod tidy

starter: generate
	@rm -rf ./starter/__genxexample && \
		mkdir -p ./starter/__genxexample && \
		go run ./cmd/genx init --target-dir=./starter/__genxexample --go-module=github.com/molon/genx/starter/__genxexample
