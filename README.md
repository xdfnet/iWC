# iCC — 微信 ↔ Claude Code 桥接工具

通过个人微信 ilink 连接 Claude Code，在微信里直接和 Claude 对话。

## 快速开始

```bash
# 1. 扫码登录微信
icc wechat setup

# 2. 启动服务
icc start

# 3. 设置开机自启（可选）
icc autostart on
```

然后从微信向你的 ilink 机器人发送消息即可。

## 安装

```bash
make install    # 编译 + 打包 + 安装到 ~/.local/bin/icc
```

或直接编译：

```bash
make build      # 编译到 build/icc
```

## 命令

| 命令 | 说明 |
|------|------|
| `icc start` | 启动服务 |
| `icc stop` | 停止服务 |
| `icc wechat setup` | 扫码登录微信 |
| `icc autostart on` | 设置开机自启（LaunchAgent） |
| `icc autostart off` | 取消开机自启 |
| `icc version` | 显示版本号 |

## 架构

```
微信 (ilink 长轮询) ──→ weixin.Platform ──→ engine ──→ claude --print
                      ←── 回复消息       ←──         ←── stdout
```

每条消息独立启动 `claude --print` 进程，输出纯文本回复，无需维护持久会话。

## 配置

配置文件: `~/.icc/config.toml`

```toml
[wechat]
token = "a8d152ac857a@im.bot:..."
base_url = "https://ilinkai.weixin.qq.com"
allow_from = "o9cq803_...@im.wechat"
long_poll_timeout_ms = 35000

[claude]
cli_path = "claude"
work_dir = "/Users/admin"
```

## 构建

```bash
make build      # go build -o build/icc .
make package    # 生成 build/icc-v0.1.0.tar.gz
make install    # build → package → cp ~/.local/bin/icc
```

## 项目结构

```
iCC/
├── main.go              # 入口、CLI 命令
├── config/config.go     # TOML 配置加载
├── weixin/
│   ├── types.go         # ilink 协议消息类型
│   ├── client.go        # HTTP 客户端
│   └── platform.go      # 微信轮询主循环
├── claude/
│   └── agent.go         # Claude Code 调用（--print 模式）
└── engine/
    └── engine.go         # 消息路由桥接
```
