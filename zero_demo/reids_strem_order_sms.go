package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
)

// Order 结构体表示订单信息
type Order struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	UserName    string  `json:"user_name"`
	TotalAmount float64 `json:"total_amount"`
	Product     string  `json:"product"`
	CreatedAt   string  `json:"created_at"`
}

// SMSEvent 表示短信事件
type SMSEvent struct {
	Type      string `json:"type"`       // 事件类型，如"marketing_sms"
	OrderID   string `json:"order_id"`   // 订单ID
	UserPhone string `json:"user_phone"` // 用户手机号
	Message   string `json:"message"`    // 短信内容
	CreatedAt string `json:"created_at"` // 创建时间
}

// Redis Stream相关配置
const (
	StreamName    = "redis_order_events"  // Redis Stream名称
	ConsumerGroup = "sms_notification_cg" // 消费者组名称
	ConsumerName  = "sms_sender"          // 消费者名称
	MaxRetries    = 3                     // 最大重试次数
)

// Redis客户端连接
var rdb *redis.Client

// 初始化Redis连接
func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis地址
		Password: "",               // Redis密码
		DB:       0,                // Redis数据库
	})

	// 测试连接
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("连接Redis失败: %v", err)
	}
	fmt.Println("成功连接到Redis")
}

// 创建消费者组
func createConsumerGroup(ctx context.Context, streamName, groupName string) error {
	_, err := rdb.XGroupCreateMkStream(ctx, streamName, groupName, "$").Result()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// 模拟下单并发送营销短信事件到Redis Stream
func placeOrderAndNotify(ctx context.Context, order Order) error {
	// 生成短信内容
	message := fmt.Sprintf("亲爱的%s，感谢您的购买！您购买的%s已成功下单，金额为%.2f元。我们还有更多优惠商品，欢迎再次光临！",
		order.UserName, order.Product, order.TotalAmount)

	// 创建短信事件
	smsEvent := SMSEvent{
		Type:      "marketing_sms",
		OrderID:   order.ID,
		UserPhone: getUserPhoneByUserID(order.UserID), // 根据用户ID获取手机号
		Message:   message,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 将事件发送到Redis Stream
	data, err := json.Marshal(smsEvent)
	if err != nil {
		return fmt.Errorf("序列化短信事件失败: %v", err)
	}

	// 添加到Redis Stream
	streamData := map[string]interface{}{
		"data": string(data),
	}

	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamName,
		Values: streamData,
	}).Result()

	if err != nil {
		return fmt.Errorf("发送短信事件到Stream失败: %v", err)
	}

	fmt.Printf("订单 %s 已创建，营销短信已加入队列\n", order.ID)
	return nil
}

// 获取用户手机号（模拟）
func getUserPhoneByUserID(userID string) string {
	// 这里应该查询数据库或缓存获取用户手机号
	// 现在只是模拟返回
	userPhones := map[string]string{
		"user001": "13800138001",
		"user002": "13800138002",
		"user003": "13800138003",
	}

	if phone, ok := userPhones[userID]; ok {
		return phone
	}
	return "13800138000" // 默认号码
}

// 消费者：处理短信发送
func smsConsumer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// 创建消费者组
	err := createConsumerGroup(ctx, StreamName, ConsumerGroup)
	if err != nil {
		log.Fatalf("创建消费者组失败: %v", err)
	}

	fmt.Printf("启动短信消费者，监听Stream: %s\n", StreamName)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("收到关闭信号，短信消费者正在退出...")
			return
		default:
			// 从Stream中读取消息
			streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: ConsumerName,
				Streams:  []string{StreamName, ">"},
				Block:    5 * time.Second, // 阻塞等待新消息，5秒超时
				Count:    1,
			}).Result()

			if err != nil {
				if err.Error() != "redis: nil" { // redis: nil 是正常的超时返回
					log.Printf("读取消息失败: %v", err)
				}
				continue
			}

			// 处理消息
			for _, stream := range streams {
				for _, msg := range stream.Messages {
					// 解析消息
					eventData, ok := msg.Values["data"].(string)
					if !ok {
						log.Printf("消息格式错误: %v", msg.Values)
						// 确认处理完成
						rdb.XAck(ctx, StreamName, ConsumerGroup, msg.ID)
						continue
					}

					var smsEvent SMSEvent
					err := json.Unmarshal([]byte(eventData), &smsEvent)
					if err != nil {
						log.Printf("解析消息失败: %v", err)
						rdb.XAck(ctx, StreamName, ConsumerGroup, msg.ID)
						continue
					}

					// 发送短信
					err = sendMarketingSMSWithRetry(smsEvent)
					if err != nil {
						log.Printf("发送短信失败，订单ID: %s, 错误: %v", smsEvent.OrderID, err)
						// 在生产环境中，这里可以将失败的消息放入死信队列
					} else {
						fmt.Printf("短信发送成功，订单ID: %s\n", smsEvent.OrderID)
					}

					// 确认消息处理完成
					_, ackErr := rdb.XAck(ctx, StreamName, ConsumerGroup, msg.ID).Result()
					if ackErr != nil {
						log.Printf("确认消息失败: %v", ackErr)
					} else {
						fmt.Printf("已确认处理短信事件: %s\n", smsEvent.OrderID)
					}
				}
			}
		}
	}
}

// 发送营销短信（带重试机制）
func sendMarketingSMSWithRetry(event SMSEvent) error {
	var lastErr error

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		err := sendMarketingSMS(event)
		if err == nil {
			return nil // 成功发送
		}

		lastErr = err
		fmt.Printf("短信发送失败，第%d次尝试，订单ID: %s，错误: %v\n", attempt, event.OrderID, err)

		if attempt < MaxRetries {
			// 等待一段时间后重试
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return lastErr
}

// 发送营销短信（模拟）
func sendMarketingSMS(event SMSEvent) error {
	fmt.Printf("\n--- 发送营销短信 ---\n")
	fmt.Printf("订单ID: %s\n", event.OrderID)
	fmt.Printf("接收号码: %s\n", event.UserPhone)
	fmt.Printf("短信内容: %s\n", event.Message)
	fmt.Printf("发送时间: %s\n", event.CreatedAt)
	fmt.Printf("-------------------\n")

	// 这里应该是调用短信服务商API的真实实现
	// 例如阿里云、腾讯云或其它短信服务
	time.Sleep(100 * time.Millisecond) // 模拟发送耗时

	fmt.Printf("营销短信已发送给: %s\n\n", event.UserPhone)
	return nil
}

// 生产环境启动函数
func startProductionEnvironment() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// 启动多个消费者协程以提高处理能力
	numConsumers := 2 // 可以根据需要调整消费者数量
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go smsConsumer(ctx, &wg)
	}

	// 处理系统信号，优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("生产环境启动完成，%d个消费者已启动\n", numConsumers)
	fmt.Println("按 Ctrl+C 关闭服务...")

	// 等待信号
	<-sigChan
	fmt.Println("\n收到关闭信号，正在优雅关闭...")

	// 取消上下文，通知所有协程退出
	cancel()

	// 等待所有消费者协程退出
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// 设置超时，防止长时间等待
	select {
	case <-done:
		fmt.Println("所有消费者已安全退出")
	case <-time.After(10 * time.Second):
		fmt.Println("等待超时，强制退出")
	}

	fmt.Println("服务已关闭")
}

// 模拟订单处理流程
func simulateOrderFlow(ctx context.Context) {
	// 创建几个测试订单
	orders := []Order{
		{
			ID:          "ORDER_001",
			UserID:      "user001",
			UserName:    "张三",
			TotalAmount: 299.99,
			Product:     "iPhone 15",
			CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
		},
		{
			ID:          "ORDER_002",
			UserID:      "user002",
			UserName:    "李四",
			TotalAmount: 199.50,
			Product:     "iPad Air",
			CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
		},
		{
			ID:          "ORDER_003",
			UserID:      "user003",
			UserName:    "王五",
			TotalAmount: 89.00,
			Product:     "AirPods Pro",
			CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
		},
	}

	// 模拟下单过程
	for _, order := range orders {
		fmt.Printf("正在处理订单: %s\n", order.ID)
		err := placeOrderAndNotify(ctx, order)
		if err != nil {
			log.Printf("处理订单失败: %v", err)
		}
		// 稍微延迟，模拟真实下单时间间隔
		time.Sleep(1 * time.Second)
	}
}

func main() {
	// 初始化Redis连接
	initRedis()

	ctx := context.Background()

	// 启动订单处理协程
	go func() {
		simulateOrderFlow(ctx)
	}()

	// time.Sleep(3 * time.Second)
	fmt.Println("...end")

	// 启动生产环境 消费者
	startProductionEnvironment()
}
