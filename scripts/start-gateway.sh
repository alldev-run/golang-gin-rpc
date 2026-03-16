#!/bin/bash

# Gateway启动脚本

echo "启动HTTP Gateway..."

# 设置配置文件路径
CONFIG_FILE=${1:-"configs/config.yaml"}

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 配置文件 $CONFIG_FILE 不存在"
    echo "请确保配置文件存在或指定正确的配置文件路径"
    echo "用法: $0 [配置文件路径]"
    exit 1
fi

# 设置环境变量
export GIN_MODE=release

# 启动gateway
echo "使用配置文件: $CONFIG_FILE"
go run examples/gateway_example.go "$CONFIG_FILE"
