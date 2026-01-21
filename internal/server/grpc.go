package server

import (
	"bffMicros/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-redis/redis/v8"
	bffV1 "github.com/wegge0857/PolyrepoGoMicros-ApiLink/bff/v1"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, bff bffV1.BffServer, redisClient *redis.Client, logger log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			// 注入你的防重复点击中间件
			PreventDuplicateMiddleware(redisClient),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	bffV1.RegisterBffServer(srv, bff)
	return srv
}
