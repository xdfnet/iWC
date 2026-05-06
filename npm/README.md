# iWC

![Version](https://img.shields.io/badge/version-1.0.14-blue)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
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

## 为什么选 iWC

| 问题 | 方案 |
|------|------|
| 手机上想问 Claude Code | 微信直接发消息，不用打开终端 |
| 多轮对话容易断上下文 | 按微信用户持久化 session，自动 resume |
| AI 处理时不知道有没有响应 | 处理期间显示“正在输入中” |
| 重启服务后状态丢失 | token、session、context token 持久化到本地 |

## 快速上手

```bash
npm i -g @xdfnet/iwc-cli
```

安装过程会下载对应平台的 `iwc` 二进制。首次没有微信配置时，会提示扫码登录。

前置要求：

- 已安装 Claude Code CLI：`npm i -g @anthropic-ai/claude`
- 已开通微信 ilink 机器人能力，并拿到可扫码登录的环境
- macOS 使用 launchd 常驻后台

## 全部命令

```bash
iwc              # 查看状态
iwc status       # 查看状态
iwc setup        # 扫码登录微信
iwc uninstall    # 卸载服务、二进制和本地配置
iwc version      # 版本
iwc help         # 帮助
```

## 工作原理

```
微信消息 → 长轮询接收 → session resume → claude --print → 微信回复
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
