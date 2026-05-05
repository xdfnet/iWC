# iWC 发布流程

## 版本号统一

1. **main.go** 中的 `const version`
2. **npm/package.json** 中的 `version`

确保两个版本号一致（如：1.0.9）

## Git Release

```bash
# 1. 清理并打包
make clean && make package

# 2. 创建 GitHub Release（上传二进制）
gh release create v1.0.9 build/iwc-darwin-arm64.tar.gz --title "v1.0.9" --notes "更新说明"
```

**注意**：二进制文件名必须是 `iwc-darwin-arm64.tar.gz`

## npm 发布

```bash
cd npm
npm publish --access public
```

## 完整发布顺序

```bash
# 1. 确认版本号统一
grep "const version" main.go
cat npm/package.json | grep version

# 2. 提交代码
git add -A && git commit -m "release: v1.0.9"
git push

# 3. 打包并创建 GitHub Release
make clean && make package
tar czf build/iwc-darwin-arm64.tar.gz -C build iwc
gh release create v1.0.9 build/iwc-darwin-arm64.tar.gz --title "v1.0.9" --notes "更新说明"

# 4. 发布 npm
cd npm
npm publish --access public

# 5. 打 Git 标签
git tag -a v1.0.9 -m "v1.0.9"
git push origin v1.0.9
```

## 测试验证

```bash
# 卸载旧版本
iwc uninstall

# 安装新版本
npm i -g @xdfnet/iwc-cli

# 验证安装后自动启动了服务
ps aux | grep iwc

# 向微信发消息测试
```

## 常见问题

**Q: npm 下载失败 404？**
A: 检查 GitHub Release 是否已创建并上传了 `iwc-darwin-arm64.tar.gz`

**Q: npm postinstall 报错？**
A: 查看日志 `/tmp/iwc_postinstall.log` 或运行 `npm i -g @xdfnet/iwc-cli 2>&1`
