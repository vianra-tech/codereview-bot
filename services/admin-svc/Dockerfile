FROM golang:1.22-alpine AS builder
WORKDIR /app
# Copy workspace and ALL service go.mod files for go work sync
COPY go.work ./
COPY services/ingress-svc/go.mod ./services/ingress-svc/
COPY services/analysis-svc/go.mod ./services/analysis-svc/
COPY services/aggregation-svc/go.mod ./services/aggregation-svc/
COPY services/integration-svc/go.mod ./services/integration-svc/
COPY services/admin-svc/go.mod ./services/admin-svc/
COPY gen/go/go.mod ./gen/go/
RUN go work sync

COPY services/admin-svc/ ./services/admin-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/admin-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /admin-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /admin-svc /admin-svc
EXPOSE 8081
ENTRYPOINT ["/admin-svc"]