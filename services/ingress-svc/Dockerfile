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

COPY services/ingress-svc/ ./services/ingress-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/ingress-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /ingress-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /ingress-svc /ingress-svc
EXPOSE 8080
ENTRYPOINT ["/ingress-svc"]