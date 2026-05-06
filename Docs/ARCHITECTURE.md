# iWC 架构

## 系统概览

```
微信 ilink → weixin.Platform → engine → claude.Agent → Claude Code
              ← 回复          ←        ← stdout
```

## 代码结构

```
iWC/
├── main.go           # CLI 入口（无参数→状态，launchd→服务）
├── config/config.go  # TOML 配置加载
├── weixin/
│   ├── types.go      # ilink 协议类型
│   ├── client.go     # HTTP 客户端（轮询/发送/扫码）
│   └── platform.go   # 消息处理、去重、token 管理
├── engine/engine.go  # 会话路由、分块发送、/new 命令
└── claude/agent.go   # Claude 进程管理（--print + --resume）
```

总计约 1200 行，零重型依赖。

## 消息流程

```
1. platform 长轮询 getupdates（35s timeout）
2. 去重 + 白名单过滤
3. engine 获取/创建 sessionID
4. agent 执行 claude --print [--resume <id>]
5. 回复分块发送（3800 字符/块）
```

## Session 多轮对话

```
首次: claude --print --session-id <new>
后续: claude --print --resume <id>
失效: agent 自动检测并重试（不带 resume）
```

## 持久化

```
~/.config/iwc/
├── config.toml          # 微信 token + Claude 路径
├── sessions.json        # userID → sessionID
├── context_tokens.json  # userID → ilink token
├── get_updates.buf      # 轮询游标
├── iwc.log              # stdout
└── iwc_error.log        # stderr
```

## CLI

- `iwc` — 查看状态
- `iwc setup` — 扫码配置
- `iwc version` — 版本号
- `iwc uninstall` — 卸载

## 发布

`make push MSG="v1.0.x 说明"` 自动完成：
递增版本 → 编译打包 → git push → GitHub Release → npm publish
