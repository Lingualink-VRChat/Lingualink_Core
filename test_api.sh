#!/bin/bash

# Lingualink Core API 测试脚本
# 测试所有主要API端点和功能

set -e

# 配置
BASE_URL="http://localhost:8080"
API_KEY="dev-key-123"
TEST_AUDIO_WAV="test/test.wav"
TEST_AUDIO_OPUS="test/test.opus"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 测试函数
test_endpoint() {
    local method=$1
    local endpoint=$2
    local description=$3
    local extra_args=${4:-""}
    
    echo
    log_info "测试: $description"
    echo "请求: $method $endpoint"
    
    if [[ "$method" == "GET" ]]; then
        response=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
            -H "X-API-Key: $API_KEY" \
            $extra_args \
            "$BASE_URL$endpoint")
    else
        response=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
            -X "$method" \
            -H "X-API-Key: $API_KEY" \
            $extra_args \
            "$BASE_URL$endpoint")
    fi
    
    # 提取HTTP状态码和响应时间
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    time_total=$(echo "$response" | grep "TIME:" | cut -d: -f2)
    response_body=$(echo "$response" | sed '/HTTP_CODE:/d' | sed '/TIME:/d')
    
    echo "状态码: $http_code"
    echo "响应时间: ${time_total}s"
    echo "响应内容:"
    echo "$response_body" | jq . 2>/dev/null || echo "$response_body"
    
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        log_success "✅ 测试通过"
    else
        log_error "❌ 测试失败 (HTTP $http_code)"
    fi
}

# 检查服务状态
check_service() {
    log_info "检查服务状态..."
    
    if curl -s "$BASE_URL/api/v1/health" > /dev/null; then
        log_success "✅ 服务正在运行"
    else
        log_error "❌ 服务未运行，请先启动服务"
        echo "启动命令: go run cmd/server/main.go"
        exit 1
    fi
}

# 检查测试文件
check_test_files() {
    log_info "检查测试文件..."
    
    if [[ -f "$TEST_AUDIO_WAV" ]]; then
        log_success "✅ 找到测试文件: $TEST_AUDIO_WAV"
        file_size=$(stat -f%z "$TEST_AUDIO_WAV" 2>/dev/null || stat -c%s "$TEST_AUDIO_WAV" 2>/dev/null || echo "unknown")
        echo "文件大小: $file_size bytes"
    else
        log_warning "⚠️  测试文件不存在: $TEST_AUDIO_WAV"
    fi
    
    if [[ -f "$TEST_AUDIO_OPUS" ]]; then
        log_success "✅ 找到测试文件: $TEST_AUDIO_OPUS"
        file_size=$(stat -f%z "$TEST_AUDIO_OPUS" 2>/dev/null || stat -c%s "$TEST_AUDIO_OPUS" 2>/dev/null || echo "unknown")
        echo "文件大小: $file_size bytes"
    else
        log_warning "⚠️  测试文件不存在: $TEST_AUDIO_OPUS"
    fi
}

# 主测试流程
main() {
    echo "=========================================="
    echo "       Lingualink Core API 测试"
    echo "=========================================="
    
    # 前置检查
    check_service
    check_test_files
    
    echo
    echo "=========================================="
    echo "           基础功能测试"
    echo "=========================================="
    
    # 1. 健康检查
    test_endpoint "GET" "/api/v1/health" "健康检查"
    
    # 2. 获取能力信息
    test_endpoint "GET" "/api/v1/capabilities" "获取系统能力"
    
    # 3. 获取支持的语言列表
    test_endpoint "GET" "/api/v1/languages" "获取支持的语言列表"
    
    # 4. 获取监控指标
    test_endpoint "GET" "/api/v1/admin/metrics" "获取监控指标"
    
    echo
    echo "=========================================="
    echo "           认证测试"
    echo "=========================================="
    
    # 5. 无认证访问（应该失败）
    log_info "测试: 无认证访问"
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}" "$BASE_URL/api/v1/capabilities")
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    if [[ "$http_code" == "401" ]]; then
        log_success "✅ 认证保护正常工作"
    else
        log_warning "⚠️  认证保护可能存在问题 (HTTP $http_code)"
    fi
    
    # 6. 错误的API Key
    log_info "测试: 错误的API Key"
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
        -H "X-API-Key: invalid-key" \
        "$BASE_URL/api/v1/capabilities")
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    if [[ "$http_code" == "401" ]]; then
        log_success "✅ API Key验证正常工作"
    else
        log_warning "⚠️  API Key验证可能存在问题 (HTTP $http_code)"
    fi
    
    echo
    echo "=========================================="
    echo "          音频处理测试"
    echo "=========================================="
    
    # 7. 表单方式上传音频（WAV）
    if [[ -f "$TEST_AUDIO_WAV" ]]; then
        test_endpoint "POST" "/api/v1/process" "音频处理 - WAV文件（表单方式）" \
            "-F 'audio=@$TEST_AUDIO_WAV' \
             -F 'task=both' \
             -F 'target_languages=英文,日文' \
             -F 'template=default'"
    fi
    
    # 8. 表单方式上传音频（OPUS）
    if [[ -f "$TEST_AUDIO_OPUS" ]]; then
        test_endpoint "POST" "/api/v1/process" "音频处理 - OPUS文件（表单方式）" \
            "-F 'audio=@$TEST_AUDIO_OPUS' \
             -F 'task=transcribe' \
             -F 'target_languages=中文' \
             -F 'user_prompt=请准确转录这段音频'"
    fi
    
    # 9. JSON方式处理音频
    if [[ -f "$TEST_AUDIO_WAV" ]]; then
        log_info "准备base64编码的音频数据..."
        audio_base64=$(base64 -i "$TEST_AUDIO_WAV" | tr -d '\n')
        
        test_endpoint "POST" "/api/v1/process/json" "音频处理 - JSON方式" \
            "-H 'Content-Type: application/json' \
             -d '{
                \"audio\": \"$audio_base64\",
                \"audio_format\": \"wav\",
                \"task\": \"translate\",
                \"target_languages\": [\"英文\"],
                \"user_prompt\": \"请将音频内容翻译成英文\"
             }'"
    fi
    
    echo
    echo "=========================================="
    echo "           错误处理测试"
    echo "=========================================="
    
    # 10. 无效的音频文件
    log_info "测试: 上传无效文件"
    echo "invalid audio content" > /tmp/invalid.txt
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
        -H "X-API-Key: $API_KEY" \
        -F 'audio=@/tmp/invalid.txt' \
        -F 'task=transcribe' \
        "$BASE_URL/api/v1/process")
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    if [[ "$http_code" == "400" ]]; then
        log_success "✅ 无效文件验证正常工作"
    else
        log_warning "⚠️  文件验证可能存在问题 (HTTP $http_code)"
    fi
    rm -f /tmp/invalid.txt
    
    # 11. 缺少必需参数
    test_endpoint "POST" "/api/v1/process/json" "缺少必需参数测试" \
        "-H 'Content-Type: application/json' \
         -d '{\"task\": \"transcribe\"}'"
    
    # 12. 无效的任务类型
    test_endpoint "POST" "/api/v1/process/json" "无效任务类型测试" \
        "-H 'Content-Type: application/json' \
         -d '{
            \"audio\": \"dGVzdA==\",
            \"audio_format\": \"wav\",
            \"task\": \"invalid_task\"
         }'"
    
    echo
    echo "=========================================="
    echo "           状态查询测试"
    echo "=========================================="
    
    # 13. 查询不存在的请求状态
    test_endpoint "GET" "/api/v1/status/non-existent-id" "查询不存在的请求状态"
    
    echo
    echo "=========================================="
    echo "           性能测试"
    echo "=========================================="
    
    # 14. 并发测试（简单版本）
    log_info "测试: 并发请求（5个并发健康检查）"
    for i in {1..5}; do
        curl -s -H "X-API-Key: $API_KEY" "$BASE_URL/api/v1/health" &
    done
    wait
    log_success "✅ 并发测试完成"
    
    echo
    echo "=========================================="
    echo "           测试总结"
    echo "=========================================="
    
    log_info "测试完成！"
    echo
    echo "💡 提示："
    echo "1. 如果LLM后端未配置，音频处理可能会失败"
    echo "2. 检查服务日志以获取详细错误信息"
    echo "3. 确保配置文件中的LLM后端地址正确"
    echo
    echo "🔧 排查命令："
    echo "- 查看服务日志: tail -f /var/log/lingualink/app.log"
    echo "- 检查配置: cat config/config.yaml"
    echo "- 测试LLM后端: curl \$VLLM_SERVER_URL/v1/models"
}

# 参数处理
case "${1:-}" in
    --help|-h)
        echo "用法: $0 [选项]"
        echo
        echo "选项:"
        echo "  --help, -h     显示帮助信息"
        echo "  --base-url     设置API基础URL (默认: http://localhost:8080)"
        echo "  --api-key      设置API密钥 (默认: dev-key-123)"
        echo
        echo "示例:"
        echo "  $0                                    # 使用默认配置运行测试"
        echo "  $0 --base-url http://localhost:8081  # 使用自定义URL"
        exit 0
        ;;
    --base-url)
        BASE_URL="$2"
        shift 2
        ;;
    --api-key)
        API_KEY="$2"
        shift 2
        ;;
esac

# 运行主程序
main "$@" 