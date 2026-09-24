FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work ./
COPY services/aggregation-svc/go.mod ./services/aggregation-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/aggregation-svc/ ./services/aggregation-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/aggregation-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /aggregation-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /aggregation-svc /aggregation-svc
ENTRYPOINT ["/aggregation-svc"]