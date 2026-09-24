module github.com/vianra/codereview/aggregation-svc

go 1.22

require (
	github.com/nats-io/nats.go v1.35.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/vianra/codereview/gen/go v0.0.0
	go.uber.org/zap v1.26.0
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/grpc v1.64.0 // indirect
)

replace github.com/vianra/codereview/gen/go => ../../gen/go
