.PHONY: generate start-database migrate serve voyager

generate:
	@go generate ./... && go mod tidy

start-database:
	@docker compose -f ./deploy/local/docker-compose.yaml --project-name genx_project up -d

migrate:
	@go run ./cmd/server migrate \
		--database-dsn="host=localhost user=genx password=genx dbname=genx port=8181 sslmode=disable TimeZone=Asia/Shanghai"

serve:
	@go run ./cmd/server serve \
		--dev-mode=true \
		--database-dsn="host=localhost user=genx password=genx dbname=genx port=8181 sslmode=disable TimeZone=Asia/Shanghai" \
		--server-allowed-origins=http://localhost:8686 \
		--server-http-address="localhost:8686"

voyager:
	docker run --rm -e GRAPHQL_ENDPOINT=http://localhost:8686/graphql -p 8585:80 graphql/voyager


