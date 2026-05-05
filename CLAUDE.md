# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

iWC = WeChat to Claude Code，微信个人号 ↔ Claude Code 桥接工具。

**定位**：小工具路线 — 够用、够小、够稳、容易看懂。

## 开发命令

```bash
# 构建和安装
make install    # 编译+安装+设置launchd开机自启+扫码配置
make package    # 打包 release 包
make build      # 仅编译到 build/iwc
make uninstall  # 卸载（停止服务+删除所有文件）

# 测试
go test ./...

# 发布（见 RELEASE.md）
```

## 架构

```
微信 (ilink 长轮询) ──→ weixin.Platform ──→ engine ──→ claude.Agent ──→ Claude Code
                      ←── 回复消息       ←──         ←── stdout
```

- `weixin/`: 微信协议客户端，长轮询接收消息，发送回复
- `engine/`: 消息路由，会话管理，多轮对话
- `claude/`: 调用 `claude --print` 进程处理消息
- `config/`: 配置加载和持久化

## 配置路径

- 配置文件: `~/.config/iwc/config.toml`
- 数据目录: `~/.config/iwc/`
- 日志: `~/.config/iwc/iwc.log`, `iwc_error.log`

## 关键设计

1. **无共享状态**: 每条消息启动独立 `claude --print` 进程
2. **session resume**: 通过 `~/.claude/sessions/` 恢复多轮对话
3. **launchd 托管**: macOS 用 launchd plist 管理进程保活
4. **软件边界清晰**: 不负责 Claude 安装，只做桥接

## 发布流程

重要！见 `RELEASE.md`：
1. 统一 main.go 和 npm/package.json 版本号
2. 创建 GitHub Release 并上传 `iwc-darwin-arm64.tar.gz`
3. npm publish
4. 打 git tag

## npm 分布

`@xdfnet/iwc-cli` 通过 npm 发布，postinstall 脚本：
1. 从 GitHub Releases 下载对应平台二进制
2. 检测配置，未配置则弹出扫码
3. 设置 launchd plist 开机自启
