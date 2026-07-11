FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY migrations ./migrations

ARG SERVICE
RUN test -n "$SERVICE" && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/app ./cmd/${SERVICE}

FROM alpine:3.22

RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
USER app

COPY --from=builder /out/app /app

EXPOSE 8080
ENTRYPOINT ["/app"]
