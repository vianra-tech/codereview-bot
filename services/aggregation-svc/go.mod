module github.com/vianra/codereview/aggregation-svc

go 1.22

require (
	github.com/nats-io/nats.go v1.35.0
	github.com/nats-io/nats.go/jetstream v0.7.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/vianra/codereview/gen/go v0.0.0
	go.uber.org/zap v1.26.0
	google.golang.org/protobuf v1.33.0
)

replace github.com/vianra/codereview/gen/go => ../../gen/go