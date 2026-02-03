# Developed by @[Viggo Van](mailto:wayne3van@gmail.com)

### 多仓库go语言微服务-bffMicros
### github.com/wegge0857/PolyrepoGoMicros-BffMicros

### 执行命令
```bash
go get github.com/wegge0857/PolyrepoGoMicros-ApiLink
go mod tidy
```
### 新增ProviderSet后，记得运行wire
```bash
cd \cmd\bffMicros\
wire
```

### 此服务无biz层、data层，可自行添加


### 添加分布式事务管理器（需要配合mysql使用）
```bash
go get github.com/dtm-labs/client
```

###### 务必把它和mysql、redis一样跑起来
###### 下载地址https://github.com/dtm-labs/dtm/releases
###### 数据库表https://github.com/dtm-labs/dtm/tree/main/sqls dtmcli.barrier.mysql.sql导入微服务数据库 dtmsvr.storage.mysql.sql导入单独的DTM库
###### http://localhost:36789/ 后台界面

###### 在对应的微服务data层
###### import "github.com/dtm-labs/client/dtmcli"
###### dtmcli.SetBarrierTableName("barrier")

###### 在dtm运行目录加入配置文件 conf.yaml:
```yaml
Store:
  Driver: 'mysql'        # 必填，指定使用 mysql
  Host: '127.0.0.1'      # 数据库地址
  Port: 3306             # 端口
  User: 'root'           # 用户名
  Password: '123123'    # 密码
  Db: 'dtm'              # 你为 DTM Server 创建的库名
```

### 运行dtm服务
```bash
.\dtm.exe -c conf.yaml
```

### 运行bff服务
```bash
go run .\cmd\etfMicros\ -conf .\configs
go run .\cmd\userMicros\ -conf .\configs

go run .\cmd\bffMicros\ -conf .\configs
```

### 请求url测试
###### 在数据库添加id为1的用户后，访问：
http://localhost:8603/api/user/1

<img width="434" height="249" alt="image" src="https://github.com/user-attachments/assets/13c934ac-1796-429f-a15a-f3f1ba8b64c3" />
<img width="452" height="253" alt="image" src="https://github.com/user-attachments/assets/d652f911-a1d9-4911-a091-a0cf82fef5d6" />


### 测试grpc
###### 可以用postman apifox 测试grpc，导入photo文件自动生成grpc接口

### 更新proto文件
```bash
kratos proto client .
```

### k3s部署命令集合
```bash
# 启动
docker build -t bff-micros:v0.0.2 . #在 bff 项目下生成镜像
docker save -o bff-micros-v0.0.2.tar bff-micros:v0.0.2 #生成镜像文件
sudo k3s ctr images import bff-micros-v0.0.2.tar #导入镜像
kubectl delete -f micros-all.yaml #清除pod
kubectl apply -f micros-all.yaml  #启动新的pod
kubectl get pods #查询当前pod
kubectl logs bff-micros-59f5d5489d-rw4m6 #查看微服务代码内部

# 维护
kubectl delete configmap bff-config # 在 bff 项目目录下，删除删除configmap
kubectl create configmap bff-config --from-file=configs/config.yaml # 生成最新configmap
kubectl exec -it bff-micros-xxx -- cat /app/configs/config.yaml #查看内部是否为最新文件
kubectl exec -it bff-micros-d49dfc78c-5zvrw -- nc -zv dtm-service 36790 # 查看容器内部 能不能 打通某个服务
```