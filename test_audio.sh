#!/bin/bash

# 音频处理专项测试脚本

BASE_URL="http://localhost:8080"
API_KEY="dev-key-123"
TEST_AUDIO_WAV="test/test.wav"
TEST_AUDIO_OPUS="test/test.opus"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "🎵 Lingualink Core 音频处理测试"
echo "================================="

# 检查服务状态
log_info "检查服务状态..."
if ! curl -s -f "$BASE_URL/api/v1/health" > /dev/null; then
    log_error "服务未运行，请先启动服务: go run cmd/server/main.go"
    exit 1
fi
log_success "服务正在运行"

# 检查测试文件
check_audio_file() {
    local file=$1
    local name=$2
    
    if [[ -f "$file" ]]; then
        size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo "0")
        log_success "找到 $name: $file (${size} bytes)"
        return 0
    else
        log_warning "$name 不存在: $file"
        return 1
    fi
}

echo
log_info "检查测试音频文件..."
wav_exists=false
opus_exists=false

if check_audio_file "$TEST_AUDIO_WAV" "WAV文件"; then
    wav_exists=true
fi

if check_audio_file "$TEST_AUDIO_OPUS" "OPUS文件"; then
    opus_exists=true
fi

if [[ "$wav_exists" == false && "$opus_exists" == false ]]; then
    log_error "没有找到测试音频文件，无法进行音频处理测试"
    exit 1
fi

# 测试音频处理
test_audio_processing() {
    local file=$1
    local format=$2
    local task=$3
    local languages=$4
    local description=$5
    
    echo
    log_info "测试: $description"
    echo "文件: $file"
    echo "任务: $task"
    echo "目标语言: $languages"
    
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
        -H "X-API-Key: $API_KEY" \
        -F "audio=@$file" \
        -F "task=$task" \
        -F "target_languages=$languages" \
        -F "template=default" \
        "$BASE_URL/api/v1/process")
    
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    time_total=$(echo "$response" | grep "TIME:" | cut -d: -f2)
    response_body=$(echo "$response" | sed '/HTTP_CODE:/d' | sed '/TIME:/d')
    
    echo "状态码: $http_code"
    echo "处理时间: ${time_total}s"
    
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        log_success "✅ 请求成功"
        echo "响应内容:"
        echo "$response_body" | jq . 2>/dev/null || echo "$response_body"
        
        # 尝试解析响应字段
        if echo "$response_body" | jq . > /dev/null 2>&1; then
            request_id=$(echo "$response_body" | jq -r '.request_id // "unknown"')
            status=$(echo "$response_body" | jq -r '.status // "unknown"')
            transcription=$(echo "$response_body" | jq -r '.transcription // ""')
            
            echo
            echo "📋 解析结果:"
            echo "- 请求ID: $request_id"
            echo "- 状态: $status"
            
            if [[ -n "$transcription" && "$transcription" != "null" && "$transcription" != "" ]]; then
                echo "- 转录内容: $transcription"
            fi
            
            # 检查翻译结果
            translations=$(echo "$response_body" | jq -r '.translations // {}')
            if [[ "$translations" != "{}" && "$translations" != "null" ]]; then
                echo "- 翻译结果:"
                echo "$response_body" | jq -r '.translations | to_entries[] | "  \(.key): \(.value)"' 2>/dev/null || echo "  解析翻译失败"
            fi
        fi
    else
        log_error "❌ 请求失败 (HTTP $http_code)"
        echo "错误信息:"
        echo "$response_body"
    fi
}

# 执行测试
echo
echo "🧪 开始音频处理测试"
echo "===================="

# 测试1: WAV文件转录
if [[ "$wav_exists" == true ]]; then
    test_audio_processing "$TEST_AUDIO_WAV" "wav" "transcribe" "中文" "WAV文件 - 仅转录"
fi

# 测试2: OPUS文件转录
if [[ "$opus_exists" == true ]]; then
    test_audio_processing "$TEST_AUDIO_OPUS" "opus" "transcribe" "中文" "OPUS文件 - 仅转录"
fi

# 测试3: 转录+翻译
if [[ "$wav_exists" == true ]]; then
    test_audio_processing "$TEST_AUDIO_WAV" "wav" "both" "英文,日文" "WAV文件 - 转录+翻译"
fi

# 测试4: 仅翻译（如果有OPUS文件）
if [[ "$opus_exists" == true ]]; then
    test_audio_processing "$TEST_AUDIO_OPUS" "opus" "translate" "英文" "OPUS文件 - 仅翻译"
fi

# JSON方式测试
if [[ "$wav_exists" == true ]]; then
    echo
    log_info "测试: JSON方式音频处理"
    
    # 编码音频文件
    log_info "正在编码音频文件为base64..."
    audio_base64=$(base64 -i "$TEST_AUDIO_WAV" | tr -d '\n')
    
    json_payload=$(cat <<EOF
{
    "audio": "$audio_base64",
    "audio_format": "wav",
    "task": "both",
    "target_languages": ["英文", "日文"],
    "user_prompt": "请准确转录并翻译这段音频内容"
}
EOF
)
    
    response=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
        -H "X-API-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$json_payload" \
        "$BASE_URL/api/v1/process/json")
    
    http_code=$(echo "$response" | grep "HTTP_CODE:" | cut -d: -f2)
    time_total=$(echo "$response" | grep "TIME:" | cut -d: -f2)
    response_body=$(echo "$response" | sed '/HTTP_CODE:/d' | sed '/TIME:/d')
    
    echo "状态码: $http_code"
    echo "处理时间: ${time_total}s"
    
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        log_success "✅ JSON方式处理成功"
        echo "$response_body" | jq . 2>/dev/null || echo "$response_body"
    else
        log_error "❌ JSON方式处理失败 (HTTP $http_code)"
        echo "$response_body"
    fi
fi

echo
echo "📊 测试总结"
echo "==========="
log_info "音频处理测试完成"
echo
echo "💡 提示："
echo "1. 如果处理失败，请检查LLM后端配置"
echo "2. 确保VLLM服务正在运行且可访问"
echo "3. 检查config/config.yaml中的后端设置"
echo
echo "🔧 排查步骤："
echo "1. 检查LLM后端状态: curl \$VLLM_SERVER_URL/v1/models"
echo "2. 查看详细日志: tail -f ./logs/app.log"
echo "3. 验证配置文件: cat config/config.yaml" 