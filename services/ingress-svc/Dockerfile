FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY services/ingress-svc/go.mod ./services/ingress-svc/
COPY gen/go/go.mod ./gen/go/

COPY services/ingress-svc/ ./services/ingress-svc/
COPY gen/go/ ./gen/go/
WORKDIR /app/services/ingress-svc
RUN CGO_ENABLED=0 GOOS=linux go build -o /ingress-svc ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /ingress-svc /ingress-svc
EXPOSE 8080
ENTRYPOINT ["/ingress-svc"]