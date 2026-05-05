# iWC 发布流程

## 版本号统一（最重要！）

发布前必须确认 `main.go` 和 `npm/package.json` 两个版本号**完全一致**。

```bash
# 检查版本号（必须一致才能继续）
grep "const version" main.go
cat npm/package.json | grep '"version"'
```

**版本号规则：**
- 每次发布必须递增，不能重复
- npm 已发布的版本**不能覆盖**，必须升级版本号

## 完整发布顺序

```bash
# 1. 确认版本号统一
grep "const version" main.go
cat npm/package.json | grep '"version"'
# 必须一致才能继续！

# 2. 本地测试 postinstall 脚本
cd npm
npm install
IWC_SKIP_DOWNLOAD=1 npm run check  # 验证脚本语法

# 3. 提交代码
cd /Users/admin/iCode/iWC
git add -A && git commit -m "release: v1.x.x"
git push

# 4. 构建并创建 GitHub Release（二进制文件名必须是 iwc-darwin-arm64.tar.gz）
go build -o build/iwc .
tar czf build/iwc-darwin-arm64.tar.gz -C build iwc
gh release create v1.x.x build/iwc-darwin-arm64.tar.gz --title "v1.x.x" --notes "更新说明"

# 5. 发布 npm
cd npm
npm publish --access public

# 6. 打 Git 标签
git tag -a v1.x.x -m "v1.x.x"
git push origin v1.x.x
```

## 测试验证

```bash
# 本地测试（不用等 npm）
# 方法1：本地编译后直接测试
go build -o /tmp/iwc-test ./main.go
/tmp/iwc-test version

# 方法2：测试 postinstall 脚本（需要先有 GitHub Release）
npm pack  # 打包本地 npm
tar -xzf *.tgz
node package/scripts/postinstall.js  # 手动运行 postinstall

# 方法3：真实安装测试
npm install -g @xdfnet/iwc-cli  # 从 npm 安装

# 卸载旧版本
iwc uninstall

# 安装新版本
npm i -g @xdfnet/iwc-cli

# 验证
ps aux | grep iwc
iwc status
```

## 常见问题

**Q: npm 下载失败 404？**
A: 检查 GitHub Release 是否存在且包含 `iwc-darwin-arm64.tar.gz`

**Q: npm postinstall 报错 ENOENT？**
A: 二进制没有正确下载到 release，检查 release 是否创建成功

**Q: npm 发布失败 "cannot publish over previously published"?**
A: npm 版本已存在，需要在 npm/package.json 中升级版本号后重新发布

**Q: release not found?**
A: `gh release create` 需要先创建 release，不能只打 git tag

**Q: chmod ENOENT 但文件存在？**
A: 可能下载还没完成就执行 chmod，检查下载是否使用 `-L` 跟随重定向

**Q: 微信没收到消息？**
A: ilink 不支持主动发消息，只能等用户发消息过来

## 经验总结

1. **release 和 tag 是两回事**：git tag 只是本地标记，GitHub Release 需要 `gh release create` 创建
2. **二进制文件名必须正确**：必须是 `iwc-darwin-arm64.tar.gz`，不是 `iwc-v1.x.x.tar.gz`
3. **npm 版本和代码版本要一致**：发布前必须检查两个版本号
4. **发布前先本地测试**：尤其是 postinstall 脚本改了之后
5. **版本号只能递增**：npm 已发布版本不能覆盖，必须用新版本号
6. **download 使用 curl**：比 https.get 更可靠，自动处理重定向
7. **launchd 环境下 PATH 不完整**：claude 路径要用 `exec.LookPath` 保存绝对路径

## 版本回滚

如果发布出问题，不要试图回滚 npm 版本（不可能），直接发布新版本修复。

```bash
# 修复问题后
# 1. 升级版本号
# 2. 重新发布
```
