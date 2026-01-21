package server

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-redis/redis/v8"
)

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
