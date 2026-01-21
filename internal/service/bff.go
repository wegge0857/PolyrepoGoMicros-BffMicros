package service

import (
	"bffMicros/internal/conf"
	"context"
	"log"

	"github.com/dtm-labs/client/dtmgrpc"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// 导入生成的 BFF 协议
	bffV1 "github.com/wegge0857/PolyrepoGoMicros-ApiLink/bff/v1"
	// 导入生成的 User 协议
	etfV1 "github.com/wegge0857/PolyrepoGoMicros-ApiLink/etf/v1"
	userV1 "github.com/wegge0857/PolyrepoGoMicros-ApiLink/user/v1"
)

const (
	etfGrpcAddr  = "127.0.0.1:9601"
	userGrpcAddr = "127.0.0.1:9602"

	etfServiceAddr  = etfGrpcAddr + "/api.etf.v1.Etf"
	userServiceAddr = userGrpcAddr + "/api.user.v1.User"
)

// 确保 BffService 实现了接口
var _ bffV1.BffServer = (*BffService)(nil)

type BffService struct {
	bffV1.UnimplementedBffServer

	userClient userV1.UserClient
	etfClient  etfV1.EtfClient

	rdb *redis.Client // 注入 Redis 客户端
}

// 1. 定义一个独立的 Redis Provider
func NewRedis(c *conf.Data) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
	})
	return rdb
}

//返回bffV1.BffServer
//符合 gRPC 生成代码的规范
//隐藏内部实现细节（封装）
func NewBffService() bffV1.BffServer { //ProviderSet
	// 测试连接
	// _, err := rdb.Ping(context.Background()).Result()
	// if err != nil {
	// 	log.Fatalf("did not connect to redis: %v", err)
	// }
	// log.Fatalln("redis connected successfully")

	// 建立连接
	connUser, err := grpc.NewClient(
		userGrpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("did not connect to user service: %v", err)
	}

	// 建立连接
	connEtf, err := grpc.NewClient(
		etfGrpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("did not connect to etf service: %v", err)
	}

	// 建议在 internal/data/data.go中添加，这里就快捷开发了
	return &BffService{
		// rdb: rdb,
		// 注入其他子服务的 grpc client (如 User, Etf)
		userClient: userV1.NewUserClient(connUser),
		etfClient:  etfV1.NewEtfClient(connEtf),
	}
}

// 加收藏 和 加用户收藏记录 （事务处理）
/**
“加收藏”和“加记录”属于两个不同的微服务，无法使用数据库本地事务。

1 分布式事务消息 (Eventual Consistency)
流程：EtfService 执行成功后，发送一个消息（Message Queue）给 UserService。
优点：性能高，解耦。缺点：记录可能会有几秒钟的延迟。

2 分布式事务管理器 - DTM(Distributed Transaction Manager)
如果你需要严格的一致性（要么都成功，要么都失败），建议集成 DTM。在 Kratos 中，DTM 支持 SAGA 或 TCC 模式。

2-1 [SAGA] 最终一致性 -> 涉及外部第三方支付、非核心链路、业务流程长
正向操作 + 补偿操作。先执行 A，成功后再执行 B；若 B 失败，则执行 A 的逆向补偿。

2-2 [TCC] 强一致性倾向 -> 库存、钱包 核心链路
Try-Confirm-Cancel。先预留资源（Try），所有人成功后确认（Confirm），否则释放（Cancel）。

2-3 [2-phase message] 性能最高 -> 跨服务的统计更新、日志记录、后续通知 -> 时效性差但在毫秒到秒级，只适合动作A触发动作B
本地事务 + 消息发送。利用 DTM 确保本地数据库操作和发送消息是原子性的。
DTM 二阶段消息，类似MQ事务消息。把 DTM Server 自身当成了一个可靠的消息中间件，所以不需要mq。

3 部署
DTM 本身是一个独立的服务。你可以在本地或服务器用 Docker 快速启动：
docker run -d --name dtm -p 36789:36789 -p 36790:36790 yedf/dtm
go get github.com/dtm-labs/client

3-1 DTM 开源地址：github.com/dtm-labs/dtm
其首创“子事务屏障” (Sub-transaction Barrier)： 这是 DTM 的核心发明。
它通过在本地数据库建立一张简单的辅助表，自动处理了分布式事务中最头疼的 幂等、空补偿、悬挂 问题。帮助实现了 防止接口被重复调用。
注意 屏障表（dtm_barrier.barrier）的主要作用是防止同一个事务 GID 因为网络抖动、超时重试而导致的重复执行。
防止重复点击 还需要 在 BFF 层做业务幂等控制，如业务唯一键；或者使用 数据库唯一索引 等。
*/
func (s *BffService) UpdateStar(ctx context.Context, req *bffV1.BffUpdateStarRequest) (*bffV1.BffUpdateStarReply, error) {
	// 1. DTM 服务器地址
	dtmServer := "localhost:36790"

	// 2. 生成一个全局事务 ID
	gid := dtmgrpc.MustGenGid(dtmServer)

	// 3. 创建 SAGA 事务
	saga := dtmgrpc.NewSagaGrpc(dtmServer, gid).
		Add( // 第一步：调用 ETF 服务增加星数，补偿操作是减少星数
			etfServiceAddr+"/UpdateStar",
			etfServiceAddr+"/UpdateStar", //补偿 在EtfService业务中 判断 dtmimp.OpCompensate dtmimp.OpRollback。或者就区分方法
			&etfV1.UpdateStarRequest{Id: req.Param.Id, Kind: "+"},
		).
		Add( // 第二步：调用用户记录服务增加记录，补偿操作是删除该记录
			userServiceAddr+"/UserStarRecord",
			userServiceAddr+"/UserStarRecord",                                        //补偿同上
			&userV1.UserStarRecordRequest{UserId: 1, EtfId: req.Param.Id, Kind: "+"}, //@Todo UserId到时候直接在jwt中获取
		)

	// 4. 提交并执行事务。 SAGA 是异步编排，请求发起者是 DTM Server，不能使用userClient等
	err := saga.Submit()
	if err != nil {
		return nil, err
	}
	// 5. 注意 幂等性 - DTM 屏障的调用 - 在各自的data层实现

	return &bffV1.BffUpdateStarReply{}, nil

}

// 获取用户信息
func (s *BffService) GetUser(ctx context.Context, req *bffV1.GetUserRequest) (*bffV1.BffUserReply, error) {
	// 调用 userMicros 的 gRPC 接口
	userResp, err := s.userClient.GetUser(ctx, &userV1.GetUserRequest{Id: req.Id})

	if err != nil {
		return &bffV1.BffUserReply{
			Code:    500,
			Message: "调用用户微服务失败: " + err.Error(),
			Data:    nil,
		}, nil
	}

	return &bffV1.BffUserReply{
		Code:    200,
		Message: "success",
		Data:    userResp, // 直接透传 user.pb.go 生成的结构体
	}, nil
}
