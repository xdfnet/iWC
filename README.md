# iWC — 微信 ↔ Claude Code 桥接工具

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)
[![Release](https://img.shields.io/github/v/release/xdfnet/iWC?style=flat-square)](https://github.com/xdfnet/iWC/releases/latest)

**小而稳，极致体验。**

iWC = **WeChat to Claude Code**。

通过个人微信 ilink 连接 Claude Code，在微信里直接和 Claude 对话。

## 快速开始

**前置要求：**

1. Node.js（macOS: `brew install node`，其他: [官网下载](https://nodejs.org/)）
2. Claude Code CLI（`npm i -g @anthropic-ai/claude`，[文档](https://docs.anthropic.com/claude-code)）
3. 微信 ilink 机器人（联系微信官方申请，启用：手机微信 → 我 → 设置 → 功能 → 插件 → 微信ClawBot → 启动）

```bash
# 一键安装（编译+安装+自启+扫码）
make install
```

然后从微信向你的 ilink 机器人发送消息即可。Claude 处理期间微信会显示”正在输入中”。

## 命令

| 命令 | 说明 |
|------|------|
| `iwc` | 查看状态 |
| `iwc setup` | 扫码登录微信 |
| `iwc uninstall` | 卸载 |
| `iwc version` | 显示版本号 |

## 架构

```
微信 (ilink 长轮询) ──→ weixin.Platform ──→ engine ──→ claude.Agent
                      ←── 回复消息       ←──         ←── stdout
```

- 每条消息启动独立 `claude --print` 进程
- session resume 支持多轮对话
- 打字状态显示”正在输入中”

## 状态持久化

`~/.config/iwc/` 下保存：

- `config.toml` — 配置
- `wechat/sessions.json` — 用户 → sessionID 映射
- `wechat/context_tokens.json` — 用户 → contextToken 映射
- `iwc.log` / `iwc_error.log` — 日志

## 安装

### 方式一：npm（推荐）

```bash
npm i -g @xdfnet/iwc-cli
```

安装时自动检测配置，未配置则弹出扫码。

### 方式二：Releases 下载

1. 下载 [Releases](https://github.com/xdfnet/iWC/releases) 中的 `iwc-v*.tar.gz`
2. 解压并将 `iwc` 二进制文件放入 `$PATH`（如 `~/.local/bin/`）
3. 确保可执行权限：`chmod +x iwc`

## License

MIT License
