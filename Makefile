# iWC Makefile
# 用 Go 构建的微信 → Claude Code 桥接工具

.PHONY: help build run dev install package clean push _require_msg _update_version

# =============================================================================
# 项目配置
# =============================================================================

PROJECT_NAME = iWC
BUILD_DIR = build
APP_NAME = iwc

# 从 main.go 动态读取版本号
VERSION = $(shell awk -F\" '/const version = / {print $$2; exit}' main.go)

# 颜色定义
RED = \033[0;31m
GREEN = \033[0;32m
YELLOW = \033[0;33m
BLUE = \033[0;34m
CYAN = \033[0;36m
NC = \033[0m

.DEFAULT_GOAL := help

# =============================================================================
# 帮助信息
# =============================================================================

help:
	@echo "$(CYAN)iWC - 微信 ↔ Claude Code 桥接工具$(NC)"
	@echo ""
	@echo "$(GREEN)核心命令:$(NC)"
	@echo "  $(YELLOW)make install$(NC)   - 一键安装（编译+安装+启动+自启）"
	@echo "  $(YELLOW)iwc$(NC)             - 查看状态"
	@echo "  $(YELLOW)iwc setup$(NC)       - 扫码登录微信"
	@echo "  $(YELLOW)iwc version$(NC)      - 显示版本号"
	@echo ""
	@echo "$(GREEN)开发命令:$(NC)"
	@echo "  $(YELLOW)make dev$(NC)        - 开发调试"
	@echo "  $(YELLOW)make build$(NC)      - 构建"
	@echo "  $(YELLOW)make push$(NC)       - 发布"
	@echo "  $(YELLOW)make clean$(NC)      - 清理"
	@echo "  $(YELLOW)make uninstall$(NC)  - 卸载"

# =============================================================================
# 构建命令
# =============================================================================

build:
	@echo "$(BLUE)构建 $(PROJECT_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) .
	@echo "$(GREEN)构建完成: $(BUILD_DIR)/$(APP_NAME)$(NC)"

dev: build
	@echo "$(BLUE)启动 $(PROJECT_NAME)...$(NC)"
	./$(BUILD_DIR)/$(APP_NAME) start

run:
	@echo "$(BLUE)运行 $(PROJECT_NAME)...$(NC)"
	go run . start

install: build
	@echo "$(BLUE)安装到 ~/.local/bin...$(NC)"
	cp $(BUILD_DIR)/$(APP_NAME) ~/.local/bin/$(APP_NAME)
	@echo "$(GREEN)安装完成: ~/.local/bin/$(APP_NAME)$(NC)"

	@echo "$(BLUE)设置开机自启...$(NC)"
	@mkdir -p ~/Library/LaunchAgents
	@printf '<?xml version="1.0" encoding="UTF-8"?>\n<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n<plist version="1.0">\n<dict>\n    <key>Label</key>\n    <string>com.user.iwc</string>\n    <key>ProgramArguments</key>\n    <array>\n        <string>%s</string>\n        <string>start</string>\n    </array>\n    <key>RunAtLoad</key>\n    <true/>\n    <key>KeepAlive</key>\n    <true/>\n    <key>StandardOutPath</key>\n    <string>%s</string>\n    <key>StandardErrorPath</key>\n    <string>%s</string>\n</dict>\n</plist>\n' "$(HOME)/.local/bin/iwc" "$(HOME)/.config/iwc/iwc.log" "$(HOME)/.config/iwc/iwc_error.log" > ~/Library/LaunchAgents/com.user.iwc.plist
	@launchctl load -w ~/Library/LaunchAgents/com.user.iwc.plist 2>/dev/null || true
	@echo "$(GREEN)开机自启已设置$(NC)"

	@echo ""
	@CONFIG_PATH="$(HOME)/.config/iwc/config.toml"; \
	if [ ! -f "$$CONFIG_PATH" ] || grep -q 'token = ""' "$$CONFIG_PATH" 2>/dev/null; then \
		echo "$(YELLOW)检测到未配置微信，开始扫码登录...$(NC)"; \
		echo ""; \
		go run . setup; \
	else \
		echo "$(GREEN)检测到已配置微信，跳过扫码$(NC)"; \
	fi
	@echo ""
	@echo "$(BLUE)启动服务...$(NC)"
	@launchctl start com.user.iwc >/dev/null 2>&1 || true
	@echo "$(GREEN)✅ 安装完成!$(NC)"
	@echo "$(CYAN)向微信发消息试试吧！$(NC)"

# =============================================================================
# 卸载命令
# =============================================================================

uninstall:
	@echo "$(BLUE)停止服务...$(NC)"
	-@launchctl unload -w ~/Library/LaunchAgents/com.user.iwc.plist 2>/dev/null || true
	@echo "$(BLUE)删除开机自启...$(NC)"
	-@rm -f ~/Library/LaunchAgents/com.user.iwc.plist
	@echo "$(BLUE)删除二进制...$(NC)"
	-@rm -f ~/.local/bin/iwc
	@echo "$(BLUE)删除配置和数据...$(NC)"
	-@rm -rf ~/.config/iwc
	@echo ""
	@echo "$(GREEN)✅ 卸载完成$(NC)"

# =============================================================================
# 打包命令
# =============================================================================

package: build
	@echo "$(BLUE)打包 $(PROJECT_NAME)...$(NC)"
	rm -f $(BUILD_DIR)/$(APP_NAME)-darwin-arm64.tar.gz
	mkdir -p $(BUILD_DIR)/pkg
	cp $(BUILD_DIR)/$(APP_NAME) $(BUILD_DIR)/pkg/iwc
	cd $(BUILD_DIR)/pkg && tar czf ../$(APP_NAME)-darwin-arm64.tar.gz iwc
	rm -rf $(BUILD_DIR)/pkg
	@echo "$(GREEN)打包完成: $(BUILD_DIR)/$(APP_NAME)-darwin-arm64.tar.gz$(NC)"

# =============================================================================
# 发布命令
# =============================================================================

_require_msg:
	@if [ -z "$(MSG)" ]; then \
		echo "$(RED)错误: 请提供提交信息$(NC)"; \
		echo "$(YELLOW)使用方法: make push MSG=\"提交信息\"$(NC)"; \
		exit 1; \
	fi

_update_version:
	@echo "$(YELLOW)递增版本号...$(NC)"
	@CURRENT=$$(awk -F\" '/const version = / {print $$2; exit}' main.go); \
	if ! echo "$$CURRENT" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "$(RED)错误: 当前版本号格式无效: $$CURRENT$(NC)"; \
		exit 1; \
	fi; \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | cut -d. -f2); \
	PATCH=$$(echo $$CURRENT | cut -d. -f3); \
	NEW_PATCH=$$((PATCH + 1)); \
	NEW_VERSION="$$MAJOR.$$MINOR.$$NEW_PATCH"; \
	echo "$(CYAN)当前版本: $$CURRENT -> 新版本: $$NEW_VERSION$(NC)"; \
	sed -i '' "s/const version = \"[^\"]*\"/const version = \"$$NEW_VERSION\"/" main.go; \
	echo "$(GREEN)main.go 版本已更新$(NC)"

push: _require_msg _update_version install package
	@echo "$(YELLOW)提交并推送...$(NC)"
	@if git diff --quiet && git diff --cached --quiet; then \
		echo "$(CYAN)没有变更需要提交$(NC)"; \
	else \
		git add .; \
		git commit -m "$(MSG)"; \
		echo "$(GREEN)提交完成: $(MSG)$(NC)"; \
		git push; \
		echo "$(GREEN)推送完成$(NC)"; \
	fi
	@echo "$(YELLOW)创建 GitHub Release...$(NC)"
	@ZIP_PATH="$(BUILD_DIR)/$(APP_NAME)-v$(VERSION).tar.gz"; \
	if [ ! -f "$$ZIP_PATH" ]; then \
		echo "$(RED)错误: 未找到发布包 $$ZIP_PATH$(NC)"; \
		exit 1; \
	fi; \
	gh release create "v$(VERSION)" "$$ZIP_PATH" --title "iWC v$(VERSION)" --notes "$(MSG)"; \
	echo "$(GREEN)已上传: $$ZIP_PATH$(NC)"; \
	echo "$(GREEN)Release 创建完成: https://github.com/xdfnet/iWC/releases/tag/v$(VERSION)$(NC)"

# =============================================================================
# 清理命令
# =============================================================================

clean:
	@echo "$(BLUE)清理构建产物...$(NC)"
	rm -rf $(BUILD_DIR)
	@echo "$(GREEN)清理完成$(NC)"
