#!/bin/bash

# Lingualink Core 文本翻译简单测试脚本

# 配置
BASE_URL="http://localhost:8080"
API_KEY="test-api-key"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "📝 Lingualink Core 文本翻译快速测试"
echo "===================================="

# 快速测试函数
quick_test() {
    local text=$1
    local languages=$2
    local description=$3

    echo
    log_info "测试: $description"
    
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
        -H "X-API-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{
            \"text\": \"$text\",
            \"target_languages\": [\"$languages\"]
        }" \
        "$BASE_URL/api/v1/process_text")

    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    response_body=$(echo "$response" | sed '/HTTP_CODE:/d')
    
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        log_success "✅ 成功 (HTTP $http_code)"
        echo "$response_body" | jq -r '.translations | to_entries[] | "  \(.key): \(.value)"' 2>/dev/null || echo "  解析失败"
    else
        log_error "❌ 失败 (HTTP $http_code)"
        echo "$response_body"
    fi
}

# 执行快速测试
quick_test "你好，世界！" "en" "中文→英文"
quick_test "Hello, world!" "zh" "英文→中文"
quick_test "こんにちは、世界！" "en" "日文→英文"

echo
log_info "快速测试完成"
