# Changelog

## v1.1.0

- **重大重构**: 重写 Claude Code 会话管理，完全对齐 cc-connect 架构
- 采用 stream-json 协议，支持持久化会话复用
- 添加 stderr 捕获和 50ms 缓冲排空宽限期，修复进程退出问题
- 支持图片消息（base64 编码 + 本地保存）
- 使用 atomic 类型替代 mutex，提升并发性能
- 精确过滤 CLAUDECODE 环境变量，避免嵌套检测

## v1.0.14

- 修复双进程问题（make install）

## v1.0.13

- 优化代码健壮性

## v1.0.12

- 初版发布
