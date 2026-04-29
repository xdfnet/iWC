# iCC Makefile
# 用 Go 构建的微信 → Claude Code 桥接工具

.PHONY: help build run dev install package clean push _require_msg _update_version

# =============================================================================
# 项目配置
# =============================================================================

PROJECT_NAME = iCC
BUILD_DIR = build
APP_NAME = icc

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
	@echo "$(CYAN)iCC - 微信 ↔ Claude Code 桥接工具$(NC)"
	@echo ""
	@echo "$(GREEN)核心命令:$(NC)"
	@echo "  $(YELLOW)dev$(NC)          - 构建并运行 (开发模式)"
	@echo "  $(YELLOW)build$(NC)        - 构建二进制到 build/"
	@echo "  $(YELLOW)install$(NC)      - 构建并安装到 ~/.local/bin"
	@echo "  $(YELLOW)package$(NC)      - 打包 tar.gz (依赖 install)"
	@echo "  $(YELLOW)push$(NC)         - 完整发布流程 (构建+安装+打包+版本更新+GitHub Release)"
	@echo "  $(YELLOW)clean$(NC)        - 清理构建产物"
	@echo ""
	@echo "$(GREEN)使用示例:$(NC)"
	@echo "  $(CYAN)make dev$(NC)                    - 开发调试"
	@echo "  $(CYAN)make install$(NC)                - 安装到 ~/.local/bin"
	@echo "  $(CYAN)make push MSG=\"修复bug\"$(NC)     - 完整发布"

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

# =============================================================================
# 打包命令
# =============================================================================

package: build
	@echo "$(BLUE)打包 $(PROJECT_NAME)...$(NC)"
	rm -f $(BUILD_DIR)/$(APP_NAME)-v$(VERSION) $(BUILD_DIR)/$(APP_NAME)-v$(VERSION).tar.gz
	cp $(BUILD_DIR)/$(APP_NAME) $(BUILD_DIR)/$(APP_NAME)-v$(VERSION)
	cd $(BUILD_DIR) && tar czf $(APP_NAME)-v$(VERSION).tar.gz $(APP_NAME)-v$(VERSION)
	rm $(BUILD_DIR)/$(APP_NAME)-v$(VERSION)
	@echo "$(GREEN)打包完成: $(BUILD_DIR)/$(APP_NAME)-v$(VERSION).tar.gz$(NC)"

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
	gh release create "v$(VERSION)" "$$ZIP_PATH" --title "iCC v$(VERSION)" --notes "$(MSG)"; \
	echo "$(GREEN)已上传: $$ZIP_PATH$(NC)"; \
	echo "$(GREEN)Release 创建完成: https://github.com/xdfnet/iCC/releases/tag/v$(VERSION)$(NC)"

# =============================================================================
# 清理命令
# =============================================================================

clean:
	@echo "$(BLUE)清理构建产物...$(NC)"
	rm -rf $(BUILD_DIR)
	@echo "$(GREEN)清理完成$(NC)"
