# --- 构建阶段 (Build Stage) ---
# 使用 alpine 版本以减小镜像体积
FROM golang:1.25-alpine AS builder

# 1. 配置 Alpine 软件源为阿里云镜像 (加速 apk add)
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 安装构建 Kratos 所需的工具
RUN apk add --no-cache make git bash

# 使用 goproxy.cn (由七牛云提供，非常稳定)
ENV GOPROXY=https://goproxy.cn,direct

# 设置工作目录
WORKDIR /app

# 1. 优先复制依赖文件，利用 Docker 缓存层
COPY go.mod go.sum ./
RUN go mod download

# 2. 复制项目源码（包含 Makefile, internal, cmd 等）
COPY . .

# 3. 执行编译
# Kratos 的 Makefile 默认通常会将产物输出到 ./bin/ 目录下
RUN make build

# --- 运行阶段 (Final Stage) ---
FROM alpine:latest

# 运行阶段同样建议更换 Alpine 源，方便以后在容器内安装调试工具
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# 安装基础运行环境，包括证书和时区数据（对微服务很重要）
RUN apk add --no-cache ca-certificates tzdata && \
    update-ca-certificates

# 设置时区为上海
ENV TZ=Asia/Shanghai

WORKDIR /app

# 4. 从构建阶段拷贝编译好的二进制文件
# 根据你的目录结构，产物名通常与项目名一致
COPY --from=builder /app/bin/bffMicros ./bffMicros

# 5. 关键：拷贝 Kratos 的配置文件目录
# Kratos 启动时必须读取 configs 文件夹
COPY --from=builder /app/configs ./configs

# 暴露 Kratos 默认端口：8000 (HTTP) 和 9000 (gRPC)
EXPOSE 8603
EXPOSE 9603
VOLUME /data/conf
# 6. 启动命令
# 使用 -conf 参数指定配置文件路径，这是 Kratos 的标准用法
# 默认用内置配置，但允许用户覆盖
ENTRYPOINT ["./bffMicros"]
CMD ["-conf", "./configs"]