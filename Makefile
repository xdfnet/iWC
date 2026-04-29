# iCC Makefile
# 用 Go 构建的微信 → Claude Code 桥接工具

.PHONY: help build run clean dev install package wechat-setup weixin-setup

# =============================================================================
# 项目配置
# =============================================================================

PROJECT_NAME = iCC
BUILD_DIR = build
APP_NAME = icc
VERSION = 0.1.0

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
	@echo "  $(YELLOW)dev$(NC)          - 开发构建并运行"
	@echo "  $(YELLOW)run$(NC)          - 直接运行 (go run)"
	@echo "  $(YELLOW)build$(NC)        - 构建二进制"
	@echo "  $(YELLOW)install$(NC)      - 构建并安装到 ~/.local/bin"
	@echo "  $(YELLOW)wechat-setup$(NC) - 扫码登录微信"
	@echo "  $(YELLOW)clean$(NC)        - 清理构建产物"
	@echo ""
	@echo "$(GREEN)使用示例:$(NC)"
	@echo "  $(CYAN)make dev$(NC)              - 开发模式"
	@echo "  $(CYAN)make wechat-setup$(NC)     - 首次配置微信连接"

build:
	@echo "$(BLUE)构建 $(PROJECT_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) .
	@echo "$(GREEN)构建完成: $(BUILD_DIR)/$(APP_NAME)$(NC)"

run:
	@echo "$(BLUE)运行 $(PROJECT_NAME)...$(NC)"
	go run . start

dev: build
	@echo "$(BLUE)启动 $(PROJECT_NAME)...$(NC)"
	./$(BUILD_DIR)/$(APP_NAME) start

install: package
	@echo "$(BLUE)安装到 ~/.local/bin...$(NC)"
	cp $(BUILD_DIR)/$(APP_NAME) ~/.local/bin/$(APP_NAME)
	@echo "$(GREEN)安装完成: ~/.local/bin/$(APP_NAME)$(NC)"

package: build
	@echo "$(BLUE)打包 $(PROJECT_NAME)...$(NC)"
	cp $(BUILD_DIR)/$(APP_NAME) $(BUILD_DIR)/$(APP_NAME)-v$(VERSION)
	cd $(BUILD_DIR) && tar czf $(APP_NAME)-v$(VERSION).tar.gz $(APP_NAME)-v$(VERSION)
	rm $(BUILD_DIR)/$(APP_NAME)-v$(VERSION)
	@echo "$(GREEN)打包完成: $(BUILD_DIR)/$(APP_NAME)-v$(VERSION).tar.gz$(NC)"

wechat-setup:
	@echo "$(BLUE)微信扫码登录配置...$(NC)"
	go run . wechat setup

weixin-setup: wechat-setup

clean:
	@echo "$(BLUE)清理...$(NC)"
	rm -rf $(BUILD_DIR)
	@echo "$(GREEN)清理完成$(NC)"
