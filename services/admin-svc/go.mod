module github.com/vianra/codereview/admin-svc

go 1.22

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/go-playground/validator/v10 v10.20.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/jackc/pgx/v5 v5.5.4
	github.com/nats-io/nats.go v1.35.0
	github.com/nats-io/nats.go/jetstream v0.6.0
	github.com/redis/go-redis/v9 v9.5.1
	github.com/stripe/stripe-go/v78 v78.0.0
	github.com/vianra/codereview/gen/go/analysis/v1 v0.0.0
	github.com/vianra/codereview/gen/go/events/v1 v0.0.0
	go.uber.org/zap v1.26.0
	google.golang.org/protobuf v1.33.0
)

replace (
	github.com/vianra/codereview/gen/go/analysis/v1 => ../../gen/go/analysis/v1
	github.com/vianra/codereview/gen/go/events/v1 => ../../gen/go/events/v1
)