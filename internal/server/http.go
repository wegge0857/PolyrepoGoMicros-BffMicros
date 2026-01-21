package server

import (
	"bffMicros/internal/conf"

	"github.com/go-redis/redis/v8"
	bffV1 "github.com/wegge0857/PolyrepoGoMicros-ApiLink/bff/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, bff bffV1.BffServer, redisClient *redis.Client, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			// 注入你的防重复点击中间件
			PreventDuplicateMiddleware(redisClient),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	bffV1.RegisterBffHTTPServer(srv, bff)
	return srv
}
