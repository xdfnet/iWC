# iCC - iCode Claude Code Connector

微信个人号 ↔ Claude Code 桥接工具。

**定位**：小工具路线 — 够用、够小、够稳、容易看懂。

## 核心能力

- 扫码配置，5 分钟跑起来
- session resume 多轮对话
- 打字状态显示"正在输入中"
- 状态持久化，重启不丢失

## 命令

```
icc start             启动服务
icc stop              停止服务
icc restart           重启服务
icc wechat setup      扫码登录微信
icc autostart on/off  开机自启
icc version            版本号
```

## 架构

```
微信 (ilink 长轮询) ──→ weixin.Platform ──→ engine ──→ claude.Agent
                      ←── 回复消息       ←──         ←── stdout
```

- 每条消息独立 `claude --print` 进程
- 通过 `--resume <session_id>` 恢复对话
- session 存 `~/.icc/wechat/sessions.json`

## 项目结构

```
iCC/
├── main.go              # 入口、CLI
├── config/config.go     # 配置加载
├── weixin/
│   ├── types.go         # 协议类型
│   ├── client.go        # HTTP 客户端
│   ├── client_test.go   # 测试
│   ├── platform.go      # 轮询 + 持久化
│   └── platform_test.go # 测试
├── claude/
│   └── agent.go         # 进程管理
└── engine/
    └── engine.go         # 消息路由
```

## 构建

```bash
make install    # 编译 + 安装
make build      # 编译到 build/icc
```

## 测试

```bash
go test ./...
```
