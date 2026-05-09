.PHONY: install build run clean test help

# 变量定义
APP_NAME := cleanmark-server
BUILD_DIR := ./cmd/server
MAIN_FILE := $(BUILD_DIR)/main.go

# 默认目标
help:
	@echo "CleanMark 去水印平台 - 构建系统"
	@echo "=============================="
	@echo ""
	@echo "可用命令:"
	@echo "  make install   - 安装依赖"
	@echo "  make build     - 编译项目"
	@echo "  make run       - 运行服务"
	@echo "  make dev       - 开发模式运行（热重载）"
	@echo "  make test      - 运行测试"
	@echo "  make clean     - 清理编译文件"
	@echo "  make docker    - 构建Docker镜像"
	@echo "  make docs      - 生成API文档"
	@echo ""

# 安装依赖
install:
	@echo "📦 安装Go依赖..."
	export GOPROXY=https://goproxy.cn,direct
	go mod tidy
	go mod download
	@echo "✅ 依赖安装完成"

# 编译项目
build:
	@echo "🔨 编译项目中..."
	mkdir -p data
	go build -o $(APP_NAME) $(MAIN_FILE)
	@echo "✅ 编译成功: $(APP_NAME)"

# 运行服务
run: build
	@echo "🚀 启动CleanMark服务..."
	@echo "访问地址: http://localhost:8080"
	./$(APP_NAME)

# 开发模式（需要安装air）
dev:
	@which air > /dev/null || (echo "请先安装air: go install github.com/air-verse/air@latest" && exit 1)
	@echo "🔧 开发模式启动（热重载）..."
	air

# 运行测试
test:
	@echo "🧪 运行测试..."
	go test -v ./...
	@echo "✅ 测试完成"

# 清理文件
clean:
	@echo "🧹 清理编译文件..."
	rm -f $(APP_NAME)
	rm -rf data/*.db
	@echo "✅ 清理完成"

# Docker构建
docker:
	@echo "🐳 构建Docker镜像..."
	docker build -t cleanmark:latest .
	@echo "✅ 镜像构建完成"

# API文档生成
docs:
	@echo "📚 生成API文档..."
	swagger init -g cmd/server/main.go -o docs --title "CleanMark API"
	@echo "✅ 文档已生成到 docs/ 目录"

# 格式化代码
fmt:
	@echo "💅 格式化代码..."
	gofmt -w .
	goimports -w .
	@echo "✅ 格式化完成"

# 代码检查
lint:
	@echo "🔍 检查代码质量..."
	golangci-lint run ./...
	@echo "✅ 检查完成"

# 数据库备份
backup:
	@echo "💾 备份数据库..."
	@if [ -f data/cleanmark.db ]; then \
		cp data/cleanmark.db backup/cleanmark_$(shell date +%Y%m%d_%H%M%S).db; \
		echo "✅ 备份成功"; \
	else \
		echo "⚠️ 数据库文件不存在"; \
	fi

# 显示版本信息
version:
	@echo "CleanMark v1.0.0"
	@echo "Go版本: $(shell go version)"
	@echo "构建时间: $(shell date)"

# 一键部署（生产环境）
deploy: build
	@echo "🚀 准备部署..."
	@echo "请确保："
	@echo "1. 已修改JWT_SECRET环境变量"
	@echo "2. 已配置HTTPS证书"
	@echo "3. 已设置数据库备份计划任务"
	@echo ""
	@echo "启动命令: nohup ./$(APP_NAME) > app.log 2>&1 &"
