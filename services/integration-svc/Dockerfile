FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/integration-svc/go.mod services/integration-svc/go.sum ./services/integration-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/integration-svc/ ./services/integration-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/integration-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /integration-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /integration-svc /integration-svc
ENTRYPOINT ["/integration-svc"]