#!/bin/bash

# Lingualink Core 音频处理简单测试脚本 - v2.0 API

# 配置
BASE_URL="http://localhost:8000"
API_KEY="lls-xxxxxs"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "🎵 Lingualink Core 音频处理快速测试 (v2.0 API)"
echo "=============================================="

# 快速测试函数
quick_test() {
    local task=$1
    local languages=$2
    local description=$3

    echo
    log_info "测试: $description"
    
    # 创建简单的测试音频数据（base64编码的"test"）
    local test_audio="dGVzdA=="
    
    local payload
    if [[ "$task" == "transcribe" ]]; then
        payload="{
            \"audio\": \"$test_audio\",
            \"audio_format\": \"wav\",
            \"task\": \"$task\"
        }"
    else
        payload="{
            \"audio\": \"$test_audio\",
            \"audio_format\": \"wav\",
            \"task\": \"$task\",
            \"target_languages\": [\"$languages\"]
        }"
    fi
    
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
        -H "X-API-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL/api/v1/process_audio")

    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    response_body=$(echo "$response" | sed '/HTTP_CODE:/d')
    
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        log_success "✅ 成功 (HTTP $http_code)"
        echo "$response_body" | jq -r '.request_id // "unknown"' | sed 's/^/  请求ID: /'
    else
        log_error "❌ 失败 (HTTP $http_code)"
        echo "$response_body" | head -3
    fi
}

# 执行快速测试
quick_test "transcribe" "" "音频转录"
quick_test "translate" "en" "音频翻译→英文"
quick_test "translate" "ja" "音频翻译→日文"

# 测试端点可访问性
echo
log_info "测试端点可访问性..."

# 测试健康检查
health_response=$(curl -s -w "\nHTTP_CODE:%{http_code}" "$BASE_URL/api/v1/health")
health_code=$(echo "$health_response" | grep "HTTP_CODE:" | cut -d: -f2)

if [[ "$health_code" == "200" ]]; then
    log_success "✅ 健康检查通过"
else
    log_error "❌ 健康检查失败 (HTTP $health_code)"
fi

# 测试文本翻译端点
text_response=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
    -H "X-API-Key: $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"text":"test","target_languages":["en"]}' \
    "$BASE_URL/api/v1/process_text")

text_code=$(echo "$text_response" | grep "HTTP_CODE:" | cut -d: -f2)
if [[ "$text_code" =~ ^[24][0-9][0-9]$ ]]; then
    log_success "✅ 文本翻译端点可访问"
else
    log_error "❌ 文本翻译端点异常 (HTTP $text_code)"
fi

echo
log_info "快速测试完成"
echo
echo "💡 提示："
echo "- 运行完整测试: ./test_audio_local.sh"
echo "- 测试文本翻译: ./test_text_local.sh"
echo "- 查看API文档: docs/API_Documentation.md"
