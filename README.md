# iWC

![Version](https://img.shields.io/badge/version-1.0.14-blue)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.22-blue)](https://golang.org/dl/)
![Platform](https://img.shields.io/badge/platform-macOS-green)

iWC 让 Claude Code 住进微信。你在微信里发消息，它在 Mac 上调用 Claude Code，处理完再把结果回到微信。

适合想把微信当作随身 AI 入口的开发者。手机上问问题，Mac 上跑 Claude Code；多轮对话自动续上，处理期间还会显示“正在输入中”。

## 效果示例

```
你在微信发：
帮我解释这个 Go 报错

iWC：
  收到微信消息
  启动 claude --print
  复用该用户的 session
  把 Claude 回复发回微信
```

常用命令：

```bash
iwc              # 查看服务状态
iwc setup        # 扫码登录微信 ilink
iwc version      # 查看版本
iwc uninstall    # 卸载服务和配置
```

## 为什么选 iWC

| 问题 | 方案 |
|------|------|
| 手机上想问 Claude Code | 微信直接发消息，不用打开终端 |
| 多轮对话容易断上下文 | 按微信用户持久化 session，自动 resume |
| AI 处理时不知道有没有响应 | 处理期间显示“正在输入中” |
| 重启服务后状态丢失 | token、session、context token 持久化到本地 |

## 快速上手

**npm 安装：**

```bash
npm i -g @xdfnet/iwc-cli
```

安装过程会下载对应平台的 `iwc` 二进制。首次没有微信配置时，会提示扫码登录。

**源码安装：**

```bash
git clone https://github.com/xdfnet/iWC.git && cd iWC && make install
```

安装后验证：

```bash
iwc
iwc version
```

前置要求：

- 已安装 Claude Code CLI：`npm i -g @anthropic-ai/claude`
- 已开通微信 ilink 机器人能力，并拿到可扫码登录的环境
- macOS 使用 launchd 常驻后台

## 工作原理

```
你在微信发消息
        │
        ▼
┌─────────────────────────────────────────────────────┐
│  iwc — Mac 上常驻的微信 ↔ Claude Code 桥接服务        │
│                                                       │
│   微信 ilink 长轮询                                  │
│         │                                            │
│         ▼                                            │
│   weixin.Platform                                    │
│   （接收消息 / 发送回复 / 打字状态）                  │
│         │                                            │
│         ▼                                            │
│   engine                                             │
│   （按用户恢复 session，串起处理流程）                │
│         │                                            │
│         ▼                                            │
│   claude.Agent                                       │
│   （启动 claude --print，读取 stdout）                │
└─────────────────────────────────────────────────────┘
```

**消息处理流程：**

```
微信消息 → 长轮询接收 → session resume → claude --print → 微信回复
```

## 全部命令

```bash
iwc              # 查看状态
iwc status       # 查看状态
iwc setup        # 扫码登录微信
iwc uninstall    # 卸载服务、二进制和本地配置
iwc version      # 版本
iwc help         # 帮助
```

## 配置说明

`~/.config/iwc/config.toml`：

```toml
[wechat]
token = "你的 ilink token"
base_url = "https://ilinkai.weixin.qq.com"
allow_from = ""
long_poll_timeout_ms = 35000

[claude]
work_dir = "/Users/admin"
cli_path = "claude"

[system]
data_dir = "/Users/admin/.config/iwc"
```

说明：

- `allow_from` 为空时允许所有微信用户；生产使用建议限制来源。
- `work_dir` 决定 Claude Code 默认在哪个目录里工作。
- `cli_path` 可填写 `claude` 或 Claude CLI 的绝对路径。
- 旧版 `~/.iwc/config.toml` 会自动迁移到 `~/.config/iwc/config.toml`。

## 开发命令

```bash
make build      # 编译 iwc
make install    # 安装并设置 launchd 自启
make dev        # 构建后前台启动
make run        # 前台运行服务
make package    # 打包 release tar.gz
make uninstall  # 卸载服务和配置
make clean      # 清理构建产物
make help       # 显示帮助
```

## 文件路径

| 文件 | 用途 |
|------|------|
| `~/Library/LaunchAgents/com.user.iwc.plist` | macOS 自动启动服务 |
| `~/.local/bin/iwc` | iWC 二进制 |
| `~/.config/iwc/config.toml` | 微信、Claude、系统配置 |
| `~/.config/iwc/wechat/sessions.json` | 微信用户到 Claude session 的映射 |
| `~/.config/iwc/wechat/context_tokens.json` | 微信用户到 context token 的映射 |
| `~/.config/iwc/iwc.log` | 标准日志 |
| `~/.config/iwc/iwc_error.log` | 错误日志 |

## License

MIT — 随便用，随便改。
