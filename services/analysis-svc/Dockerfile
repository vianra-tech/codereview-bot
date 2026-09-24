FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git gcc musl-dev
WORKDIR /app
COPY services/analysis-svc/go.mod services/analysis-svc/go.sum ./services/analysis-svc/
COPY gen/go/go.mod ./gen/go/

COPY services/analysis-svc/ ./services/analysis-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/analysis-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /analysis-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates git
COPY --from=builder /analysis-svc /analysis-svc
WORKDIR /tmp/codereview
ENTRYPOINT ["/analysis-svc"]