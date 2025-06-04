#!/bin/bash

# Lingualink Core 文本翻译测试脚本
# 用于测试 /api/v1/process_text 端点

# 配置
BASE_URL="https://api2.lingualink.aiatechco.com"
API_KEY="lls-2f5v4Mai6cRvVMNTjiQH"

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

echo "📝 Lingualink Core 文本翻译测试"
echo "================================="

# 检查依赖工具
log_info "检查依赖工具..."
if ! command -v jq &> /dev/null; then
    log_error "jq 未安装，请先安装: sudo apt-get install jq 或 brew install jq"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    log_error "curl 未安装，请检查系统环境"
    exit 1
fi
log_success "依赖工具检查完成"

# 检查服务状态
log_info "检查服务状态..."
if ! curl -s -f "$BASE_URL/api/v1/health" > /dev/null; then
    log_error "服务未运行，请先启动服务: go run cmd/server/main.go"
    exit 1
fi
log_success "服务正在运行"

# 测试文本翻译
test_text_translation() {
    local text=$1
    local languages=$2
    local description=$3
    local source_lang=$4

    echo
    log_info "测试: $description"
    echo "文本: $text"
    echo "目标语言: $languages"
    if [[ -n "$source_lang" ]]; then
        echo "源语言: $source_lang"
    fi

    # 构建JSON请求体
    local temp_json=$(mktemp)
    
    # 处理target_languages
    local lang_array
    if [[ -z "$languages" ]]; then
        # 如果没有指定语言，使用空数组
        lang_array="[]"
    elif [[ "$languages" == *","* ]]; then
        # 多个语言，用逗号分隔
        IFS=',' read -ra LANG_ARRAY <<< "$languages"
        lang_array=$(printf '%s\n' "${LANG_ARRAY[@]}" | jq -R . | jq -s .)
    else
        # 单个语言
        lang_array=$(jq -n --arg lang "$languages" '[$lang]')
    fi

    # 构建JSON请求
    if [[ -n "$source_lang" ]]; then
        jq -n \
            --arg text "$text" \
            --arg source_language "$source_lang" \
            --argjson target_languages "$lang_array" \
            '{
                text: $text,
                source_language: $source_language,
                target_languages: $target_languages
            }' > "$temp_json"
    else
        jq -n \
            --arg text "$text" \
            --argjson target_languages "$lang_array" \
            '{
                text: $text,
                target_languages: $target_languages
            }' > "$temp_json"
    fi

    response=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
        -H "X-API-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        -d @"$temp_json" \
        "$BASE_URL/api/v1/process_text")

    # 清理临时文件
    rm -f "$temp_json"
    
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
            source_text=$(echo "$response_body" | jq -r '.source_text // ""')
            
            echo
            echo "📋 解析结果:"
            echo "- 请求ID: $request_id"
            echo "- 状态: $status"
            
            if [[ -n "$source_text" && "$source_text" != "null" && "$source_text" != "" ]]; then
                echo "- 源文本: $source_text"
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
echo "🧪 开始文本翻译测试"
echo "===================="

# 测试1: 中文翻译成英文
test_text_translation "你好，世界！这是一个测试文本。" "en" "中文翻译成英文"

# 测试2: 中文翻译成日文
test_text_translation "今天天气很好，我们去公园散步吧。" "ja" "中文翻译成日文"

# 测试3: 中文翻译成多种语言
test_text_translation "人工智能正在改变我们的生活方式。" "en,ja" "中文翻译成多种语言"

# 测试4: 英文翻译成中文
test_text_translation "Hello, this is a test message for translation." "zh" "英文翻译成中文" "en"

# 测试5: 日文翻译成中文和英文
test_text_translation "こんにちは、これはテストメッセージです。" "zh,en" "日文翻译成中文和英文" "ja"

# 测试6: 长文本翻译
test_text_translation "在当今快速发展的数字化时代，人工智能技术已经深入到我们生活的方方面面。从智能手机的语音助手到自动驾驶汽车，从医疗诊断到金融分析，AI正在重塑着各个行业的运作模式。" "en" "长文本翻译测试"

# 测试7: 繁体中文翻译
test_text_translation "科技创新推动社会进步。" "zh-hant" "简体中文翻译成繁体中文"

# 测试8: 专业术语翻译
test_text_translation "机器学习是人工智能的一个重要分支，它使计算机能够在没有明确编程的情况下学习和改进。" "en,ja" "专业术语翻译测试"

# 测试9: 空文本错误测试
test_text_translation "" "en" "空文本错误测试"

# 测试10: 无目标语言错误测试
test_text_translation "这是一个错误测试。" "" "无目标语言错误测试"

# 测试11: 音频转录功能验证（通过process_audio端点）
echo
echo "🎵 音频处理端点功能验证"
echo "========================"
log_info "验证 /process_audio 端点的 transcribe 和 translate 功能"

# 注意：这里只是展示API调用格式，实际需要真实的音频数据
echo "转录任务示例："
echo 'curl -X POST \'
echo '  -H "X-API-Key: your-api-key" \'
echo '  -H "Content-Type: application/json" \'
echo '  -d '"'"'{'
echo '    "audio": "base64-audio-data",'
echo '    "audio_format": "wav",'
echo '    "task": "transcribe"'
echo '  }'"'"' \'
echo '  "http://localhost:8080/api/v1/process_audio"'

echo
echo "翻译任务示例："
echo 'curl -X POST \'
echo '  -H "X-API-Key: your-api-key" \'
echo '  -H "Content-Type: application/json" \'
echo '  -d '"'"'{'
echo '    "audio": "base64-audio-data",'
echo '    "audio_format": "wav",'
echo '    "task": "translate",'
echo '    "target_languages": ["en", "ja"]'
echo '  }'"'"' \'
echo '  "http://localhost:8080/api/v1/process_audio"'

echo
echo "📊 测试总结"
echo "==========="
log_info "文本翻译测试完成"
echo
echo "💡 提示："
echo "1. 如果翻译失败，请检查LLM后端配置"
echo "2. 确保VLLM服务正在运行且可访问"
echo "3. 检查config/config.yaml中的后端设置"
echo "4. 文本长度限制为10000字符"
echo
echo "🔧 排查步骤："
echo "1. 检查LLM后端状态: curl \$VLLM_SERVER_URL/v1/models"
echo "2. 查看详细日志: tail -f ./logs/app.log"
echo "3. 验证配置文件: cat config/config.yaml"
echo "4. 测试健康检查: curl $BASE_URL/api/v1/health"
