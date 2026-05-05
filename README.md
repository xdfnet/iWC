# iWC — 微信 ↔ Claude Code 桥接工具

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)
[![Release](https://img.shields.io/github/v/release/xdfnet/iWC?style=flat-square)](https://github.com/xdfnet/iWC/releases/latest)

iWC = **WeChat to Claude Code**。

通过个人微信 ilink 连接 Claude Code，在微信里直接和 Claude 对话。

**小工具路线**：够用、够小、够稳、容易看懂。

## 快速开始

```bash
# 1. 一键安装
make install

# 2. 扫码登录微信（如需要）
iwc setup
```

安装过程自动完成：编译 → 安装 → 设置自启 → 启动服务 → 扫码配置。

然后从微信向你的 ilink 机器人发送消息即可。Claude 处理期间微信会显示"正在输入中"。

## 命令

| 命令 | 说明 |
|------|------|
| `iwc` | 查看状态 |
| `iwc setup` | 扫码登录微信 |
| `iwc version` | 显示版本号 |

## 架构

```
微信 (ilink 长轮询) ──→ weixin.Platform ──→ engine ──→ claude.Agent
                      ←── 回复消息       ←──         ←── stdout
```

- 每条消息启动独立 `claude --print` 进程
- session resume 支持多轮对话
- 打字状态显示"正在输入中"

## 状态持久化

`~/.iwc/wechat/` 下保存：

- `sessions.json` — 用户 → sessionID 映射
- `context_tokens.json` — 用户 → contextToken 映射
- `get_updates.buf` — 轮询游标

## 配置

`~/.iwc/config.toml`：

```toml
[wechat]
token = "a8d152ac857a@im.bot:..."
base_url = "https://ilinkai.weixin.qq.com"
allow_from = ""
long_poll_timeout_ms = 35000

[claude]
cli_path = "claude"
work_dir = "/Users/admin"

[system]
data_dir = "/Users/admin/.iwc"
```

## 安装

### 方式零：npm 安装（推荐给使用者）

```bash
npm i -g @xdfnet/iwc-cli
iwc version
```

当前 npm 安装链路已在 `darwin-arm64`（Apple Silicon macOS）完成实测。  
若你是其他平台，请先确认对应平台二进制资产已发布到 Releases（见下方“npm 发布（维护者）”）。

### 方式一：Releases 下载（推荐）

1. 下载 [Releases](https://github.com/xdfnet/iWC/releases) 中的 `iwc-v*.tar.gz`
2. 解压并将 `iwc` 二进制文件放入 `$PATH`（如 `~/.local/bin/`）
3. 确保可执行权限：`chmod +x iwc`

### 方式二：源码编译

```bash
git clone https://github.com/xdfnet/iWC.git
cd iWC
make install
```

## npm 发布（维护者）

仓库内已提供 npm 分发包目录：`npm/`。

1. 确保 GitHub Release（如 `v1.0.6`）包含以下资产：
- `iwc-darwin-arm64.tar.gz`
- `iwc-darwin-amd64.tar.gz`
- `iwc-linux-arm64.tar.gz`
- `iwc-linux-amd64.tar.gz`
- `iwc-windows-amd64.zip`
- `iwc-windows-arm64.zip`

目前已验证并发布：`iwc-darwin-arm64.tar.gz`（对应 npm `@xdfnet/iwc-cli@1.0.6`）。

2. 每个压缩包根目录应包含可执行文件：
- macOS/Linux: `iwc`
- Windows: `iwc.exe`

3. 发布 npm 包：

```bash
cd npm
npm publish --access public
```

## 测试

```bash
go test ./...
```

## 项目结构

```
iWC/
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

## License

MIT License
