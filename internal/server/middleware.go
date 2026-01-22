package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http" // 必须引入这个
	"github.com/go-redis/redis/v8"
)

// 黑名单控制
var blacklist = map[string]bool{
	"192.168.1.1": true,
	"192.168.1.2": true,
}

func BlacklistMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			if info, ok := transport.FromServerContext(ctx); ok {
				// 关键点：断言为 HTTP 的 Transport
				if ht, ok := info.(*http.Transport); ok {

					// 现在可以访问原始的 http.Request 对象了
					clientIP := GetClientIP(ht.Request()) // 调用上面的工具函数

					if blacklist[clientIP] {
						//@TODO: 建议使用 proto 生成的错误码或自定义错误 前端端更好地识别
						return nil, errors.Forbidden("ACCESS_DENIED", "IP blocked")
					}
				}
			}
			return handler(ctx, req)
		}
	}
}

// 防重复请求
func PreventDuplicateMiddleware(redisClient *redis.Client) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			// 1. 提取 UID 和 业务标识 (例如：UpdateStar)
			uid := 10000000 //@TODO

			if uid >= 0 {
				lockKey := fmt.Sprintf("lock-pre-dup:%d", uid)

				// 2. 尝试抢锁 (SET lockKey 1 NX EX 3)
				ok, err := redisClient.SetNX(ctx, lockKey, 1, 3*time.Second).Result()
				if err != nil {
					return nil, errors.InternalServer("REDIS_ERROR", "system busy")
				}

				if !ok {
					// 3. 抢锁失败，说明 3 秒内点过了
					return nil, errors.Forbidden("TOO_MANY_REQUESTS", "请勿重复点击")
				}
			}

			return handler(ctx, req)
		}
	}
}

// 获取客户端 IP
func GetClientIP(r *http.Request) string {
	// 1. 优先从 X-Forwarded-For 读取（可能包含多个 IP，取第一个）
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// 2. 其次尝试 X-Real-IP
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return xRealIP
	}

	// 3. 最后兜底才使用 RemoteAddr
	// 注意：RemoteAddr 包含端口号 (如 "192.168.1.1:5678")，需要分割
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
