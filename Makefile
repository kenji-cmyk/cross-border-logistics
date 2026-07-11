BACKEND_DIR := backend

.PHONY: fmt test vet build compose-up compose-down compose-logs demo reset-demo

fmt:
	cd $(BACKEND_DIR) && gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	cd $(BACKEND_DIR) && go test ./...

vet:
	cd $(BACKEND_DIR) && go vet ./...

build:
	cd $(BACKEND_DIR) && go build ./...

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f

demo:
	bash scripts/demo.sh

reset-demo:
	bash scripts/reset-demo.sh
