# Developed by @Viggo Van
[Email](mailto:wayne3van@gmail.com)

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
##### 在数据库添加id为1的用户后，访问：
http://localhost:8600/api/user/1

### 可以用postman apifox 测试grpc，导入photo文件自动生成grpc接口

