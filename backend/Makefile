SERVICES := quotation-service order-service payment-service warehouse-service admin-service

.PHONY: run build test fmt vet compose-up compose-down compose-logs demo

run:
	go run ./cmd/quotation-service

build:
	@for service in $(SERVICES); do go build -o bin/$$service ./cmd/$$service || exit 1; done

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

compose-up:
	docker compose -f ../compose.yaml --project-directory .. up -d --build

compose-down:
	docker compose -f ../compose.yaml --project-directory .. down

compose-logs:
	docker compose -f ../compose.yaml --project-directory .. logs -f

demo:
	@echo "The end-to-end demo script is added in Phase 8."
