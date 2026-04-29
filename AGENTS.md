# iCC - iCode Claude Code Connector

微信个人号 ↔ Claude Code 桥接工具。通过 ilink 协议连接微信，将消息转发给本地运行的 Claude Code。

## 命令

```
icc start             启动服务
icc stop              停止服务
icc wechat setup      扫码登录微信
icc autostart on      设置开机自启
icc autostart off     取消开机自启
icc version           显示版本号
```

## 架构

```
微信 (ilink 长轮询) ──→ weixin.Platform ──→ engine ──→ claude --print
                      ←── 回复消息       ←──         ←── stdout
```

每条消息独立启动 `claude --print` 进程。

## 项目结构

```
iCC/
├── main.go              # 入口, CLI 命令处理
├── config/config.go     # TOML 配置加载
├── weixin/
│   ├── types.go         # ilink 协议消息类型
│   ├── client.go        # HTTP 客户端 (getUpdates/sendMessage)
│   └── platform.go      # 微信轮询主循环
├── claude/
│   └── agent.go         # Claude Code 进程管理
└── engine/
    └── engine.go        # 消息路由桥接层
```

## 构建

```bash
make build      # go build -o build/icc .
make package    # 生成 tar.gz
make install    # 安装到 ~/.local/bin/icc
```
