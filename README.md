# sdkit-go

`sdkit-go` 是一组可复用的 Go 后端基础能力包，面向 `sdbase-go`、`sdbase-lite` 和其他 Go 后端项目。它将配置、日志、数据访问、异步任务、安全和通信等能力集中维护，供消费方按需接入。

模块路径：`github.com/huwenlong92/sdkit-go`。

首个迁移版本 `v0.1.28` 以旧 `sdkit` 的同名 tag 为源码基线，仅调整 module/import 路径。旧仓库和旧 tag 保留。消费方必须显式引用固定版本，并提交 `go.mod`、`go.sum`；升级时统一调整 import，禁止在同一接入链路混用新旧底座类型。正式构建必须移除临时本地 `replace`，使用 `-mod=readonly` 验证依赖。

## 项目定位

- 提供公共契约、具体驱动和运行时生命周期管理。
- 支持按模块直接使用，也支持通过 Runtime capability 统一初始化和回收资源。
- 业务模型、业务路由、服务组合与部署配置由消费方维护。

`core/` 和 `pkg/` 同属一个 Go module，两层均可被外部项目导入。通常从 `core/` 的公共 API 接入框架能力，从 `pkg/` 选择具体驱动或使用独立工具。

## 主要能力

| 能力 | 模块入口 |
| --- | --- |
| 配置加载与拆分配置 | [core/config](core/config) |
| 日志、链路追踪与请求标识 | [core/logger](core/logger)、[core/tracing](core/tracing)、[core/tracking](core/tracking)、[core/requestid](core/requestid) |
| GORM（PostgreSQL / SQLite）、PGX、Redis 与缓存 | [core/database](core/database)、[core/redis](core/redis)、[core/cache](core/cache) |
| 队列、定时任务与事件总线 | [core/queue](core/queue)、[core/crontab](core/crontab)、[core/eventbus](core/eventbus) |
| 鉴权、权限、安全与限流 | [core/auth](core/auth)、[core/casbin](core/casbin)、[core/security](core/security)、[core/ratelimit](core/ratelimit) |
| 实时通信与 Gin 接入 | [core/realtime](core/realtime)、[core/gin](core/gin) |
| 文件存储、邮件与短信 | [core/storage](core/storage)、[core/email](core/email)、[core/sms](core/sms) |
| 支付能力与渠道适配 | [core/payment](core/payment)、[pkg/payment](pkg/payment) |
| 系统探针与隔离代码执行 | [core/sysprobe](core/sysprobe)、[core/sandbox](core/sandbox) |
| 应用运行时与进度报告 | [core/runtime](core/runtime)、[core/reporter](core/reporter) |
| HTTP 请求、ID 与媒体工具 | [pkg/request](pkg/request)、[pkg/id](pkg/id)、[pkg/hashid](pkg/hashid)、[pkg/media](pkg/media) |
| SDIngest Go SDK | [pkg/sdingest](pkg/sdingest) |

## 环境与安装

当前 `go.mod` 要求 Go **1.27.3 或更高版本**。

在消费方项目中添加依赖：

```bash
go get github.com/huwenlong92/sdkit-go@v0.1.28
```

外部服务按所用模块准备：例如数据库、Redis、NATS、对象存储或 Docker。下面的最小示例只使用配置和日志，无需启动这些服务。

## 最小接入示例

在消费方项目中创建 `config.yaml`：

```yaml
app:
  name: demo
  mode: dev
log:
  level: info
```

创建 `main.go`，由应用定义配置结构并处理初始化错误：

```go
package main

import (
	"fmt"
	"os"

	"github.com/huwenlong92/sdkit-go/core/config"
	"github.com/huwenlong92/sdkit-go/core/logger"
	"go.uber.org/zap"
)

type Config struct {
	App struct {
		Name string `mapstructure:"name"`
		Mode string `mapstructure:"mode"`
	} `mapstructure:"app"`
	Log struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"log"`
}

func main() {
	var cfg Config
	if err := config.Load("config.yaml", &cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.App.Name, cfg.Log.Level, cfg.App.Mode); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("application initialized", zap.String("app", cfg.App.Name))
}
```

在该项目目录运行：

```bash
go run .
```

`dev` 模式下日志同时输出到控制台，文件日志默认写入 `logs/`。更完整的应用可以通过各模块的 `facade` 接入 `core/runtime`，统一管理启动、依赖和停止过程。

## Runtime 能力接入与所有权

`Use(...)` 和既有 `From` 入口继续保留。新代码可使用 `runtime.ResolveCapability[T](serviceContext, name)` 获取必需能力，或 `OptionalCapability[T]` 区分缺失与类型错误；`ResolveAppCapability[T]` 另保留初始化失败原因。服务本地名称优先于共享名称，解析结果为借用资源。

Queue 可通过 `producer.Resolve(serviceContext)` 获取投递接口，通过 `operations.Resolve(serviceContext)` 获取运维接口。Crontab 的 `ResolveOperations(serviceContext)` 返回不包含启动和关闭方法的管理接口；原 `Service` 接口保持兼容。Crontab 解锁错误会与执行错误一起返回；执行成功但解锁失败时，最终状态为失败。

创建资源的 capability 负责关闭；`WithDatabase`、`WithProducer` 注入的资源由调用者关闭。`LocalCapabilityRegistry.Set` 保留原有所有权语义，Runtime 注入使用 `SetBorrowed`，同一可比较实例的别名只关闭一次。同名替换只更新查询结果，已由 `Set` 接管的旧资源保留到 registry 关闭时按逆序回收；后续 `SetBorrowed` 不撤销已有所有权。显式指定 App 查询数据库时，不再回退到另一个 App 的默认实例。

初始化失败会回滚已成功注册的资源。重复停止保留首次关闭错误；忽略 context 的用户回调无法被 Go 强制终止。`Health.Ready` 与 `StatusRunning` 分开，阻塞服务只有实现 `ReadinessProvider` 并关闭就绪通道后才报告 ready；旧服务的 readiness 未知时保持 false。

Session 的 Set Hook 在凭据持久化前执行，失败清空会话并发送过期 Cookie；Hook 应使用传入 Session 的暂存值，不能依赖凭据已写入响应。多个 Session Hook 按顺序执行并在首个错误处停止；跨 Redis 与 Cookie 的副作用不具备分布式事务保证。Auth 的刷新失败会执行全部失败清理，保留原始错误和清理错误。

## 目录结构

```text
core/       公共契约、运行时、生命周期、框架入口与中间件
pkg/        具体实现、驱动、适配器、服务 SDK 与独立工具
tests/      按模块组织的测试
examples/   示例应用
```

隔离代码执行示例位于 [examples/sandbox-demo](examples/sandbox-demo)。

## 本地开发与验证

在消费方项目中使用本地仓库进行联调：

```bash
go mod edit -replace github.com/huwenlong92/sdkit-go=../sdkit-go
go mod tidy
```

`../sdkit-go` 应替换为实际的本地仓库路径。此替换仅用于本地联调，交付前必须移除并恢复公开固定版本：

```bash
go mod edit -dropreplace github.com/huwenlong92/sdkit-go
go mod tidy
go build -mod=readonly ./...
```

在 sdkit-go 仓库根目录运行全量测试：

```bash
go test ./...
```

开发时可先验证受影响的模块，例如：

```bash
go test ./tests/core/config ./tests/core/logger/...
```

新增测试统一放在根目录 `tests/` 下，按模块组织。涉及外部服务的测试需要按对应模块准备运行环境。
