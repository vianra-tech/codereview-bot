module github.com/vianra/codereview/integration-svc

go 1.22

require (
	github.com/bradleyfalzon/ghinstallation/v2 v2.5.0
	github.com/google/go-github/v60 v60.0.0
	github.com/nats-io/nats.go v1.35.0
	github.com/nats-io/nats.go/jetstream v0.7.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/vianra/codereview/gen/go v0.0.0
	go.uber.org/zap v1.26.0
	google.golang.org/protobuf v1.33.0
)

replace github.com/vianra/codereview/gen/go => ../../gen/go