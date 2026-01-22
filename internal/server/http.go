package server

import (
	"bffMicros/internal/conf"
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/handlers"
	bffV1 "github.com/wegge0857/PolyrepoGoMicros-ApiLink/bff/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// 严格按照这个签名写
var myRecoveryHandler recovery.HandlerFunc = func(ctx context.Context, req, err any) error {
	fmt.Printf("灾难级错误: %v\n", err)
	// 将 any 类型转换为 error 类型
	var errorValue error
	if err != nil {
		switch v := err.(type) {
		case error:
			errorValue = v
		default:
			errorValue = fmt.Errorf("%v", v)
		}
	}
	return errorValue
}

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, bff bffV1.BffServer, redisClient *redis.Client, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		// 1. Filter 层：处理底层的 HTTP 通信细节（如跨域）
		http.Filter(
			handlers.CORS(
				// 1. 允许任何域名访问
				handlers.AllowedOrigins([]string{"*"}),
				// 2. 允许任何标准的和自定义的 Header
				handlers.AllowedHeaders([]string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"}),
				// 3. 允许所有主流 HTTP 方法
				handlers.AllowedMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
				// 4. 暴露自定义 Header 给前端（可选）
				handlers.ExposedHeaders([]string{"X-Total-Count"}),
				// handlers.AllowCredentials(), // AllowedOrigins明确域名才能使用
			),
			// handlers.CompressHandler, //层开启 Gzip 压缩 默认会跳过常见的压缩格式 如图片/视频/PDF
		),
		// 2. Middleware 层：处理业务逻辑增强（如日志、鉴权）
		http.Middleware(
			// 拦截崩溃 (Panic)
			recovery.Recovery(
				recovery.WithHandler(myRecoveryHandler), // myRecoveryHandler

			),
			// Kratos 官方提供了内置的 ratelimit 中间件。默认 BBR 算法，自动保护系统不被冲垮
			ratelimit.Server(),
			// 注入你的防重复点击中间件
			PreventDuplicateMiddleware(redisClient),
			// 添加黑名单
			BlacklistMiddleware(),
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
