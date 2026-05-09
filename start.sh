#!/bin/bash

echo "🚀 CleanMark 去水印平台 - 启动脚本"
echo "======================================"

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未检测到Go环境"
    echo "请先安装Go: https://golang.org/dl/"
    exit 1
fi

# 设置国内代理
export GOPROXY=https://goproxy.cn,direct

# 创建数据目录
mkdir -p data

# 编译项目
echo "📦 正在编译项目..."
go build -o cleanmark-server ./cmd/server/main.go

if [ $? -eq 0 ]; then
    echo "✅ 编译成功!"
else
    echo "❌ 编译失败"
    exit 1
fi

# 启动服务
echo ""
echo "🎬 启动CleanMark服务..."
echo "访问地址: http://localhost:8080"
echo "API文档: http://localhost:8080/health"
echo ""
echo "按 Ctrl+C 停止服务"
echo ""

./cleanmark-server
