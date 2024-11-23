.PHONY: generate starter

generate:
	@go generate ./... && go mod tidy

starter: generate
	@rm -rf ./starter/__example && \
		mkdir -p ./starter/__example && \
		go run ./cmd/genx init --target-dir=./starter/__example --go-module=github.com/molon/genx/starter/__example
