FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.work ./
COPY services/admin-svc/go.mod services/admin-svc/go.sum ./services/admin-svc/
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