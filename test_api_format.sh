#!/bin/bash

# 简单的API格式测试脚本
# 用于验证JSON API格式是否正确

BASE_URL="https://api2.lingualink.aiatechco.com"
API_KEY="lls-2f5v4Mai6cRvVMNTjiQH"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "🧪 API格式测试"
echo "==============="

# 检查依赖
if ! command -v jq &> /dev/null; then
    log_error "需要安装 jq: sudo apt-get install jq"
    exit 1
fi

# 测试健康检查
log_info "测试健康检查..."
health_response=$(curl -s "$BASE_URL/api/v1/health")
if echo "$health_response" | jq . > /dev/null 2>&1; then
    log_success "健康检查API正常"
    echo "$health_response" | jq .
else
    log_error "健康检查API异常"
    echo "$health_response"
fi

echo

# 测试能力查询
log_info "测试能力查询..."
capabilities_response=$(curl -s -H "X-API-Key: $API_KEY" "$BASE_URL/api/v1/capabilities")
if echo "$capabilities_response" | jq . > /dev/null 2>&1; then
    log_success "能力查询API正常"
    echo "$capabilities_response" | jq .
else
    log_error "能力查询API异常"
    echo "$capabilities_response"
fi

echo

# 测试语言列表
log_info "测试语言列表..."
languages_response=$(curl -s -H "X-API-Key: $API_KEY" "$BASE_URL/api/v1/languages")
if echo "$languages_response" | jq . > /dev/null 2>&1; then
    log_success "语言列表API正常"
    echo "$languages_response" | jq .
else
    log_error "语言列表API异常"
    echo "$languages_response"
fi

echo

# 测试音频处理API格式（不发送真实音频）
log_info "测试音频处理API格式..."

# 构建测试JSON（使用假的base64数据）
test_json=$(jq -n '{
    audio: "UklGRiQAAABXQVZFZm10IBAAAAABAAEA",
    audio_format: "wav",
    task: "transcribe"
}')

echo "发送的JSON格式:"
echo "$test_json" | jq .

# 发送请求（预期会失败，但可以验证格式）
process_response=$(curl -s \
    -H "X-API-Key: $API_KEY" \
    -H "Content-Type: application/json" \
    -d "$test_json" \
    "$BASE_URL/api/v1/process")

echo "响应:"
echo "$process_response" | jq . 2>/dev/null || echo "$process_response"

log_info "API格式测试完成"
