#!/bin/bash
# 快速性能测试（单模式：Tool Use Pipeline）

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Lingualink 快速性能测试${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查测试文件
if [ ! -f "test/test.opus" ]; then
    echo -e "${RED}错误: test/test.opus 不存在${NC}"
    exit 1
fi

# 编码音频
AUDIO_BASE64=$(base64 -w 0 test/test.opus)

# 请求 JSON
REQUEST='{
  "audio": "'$AUDIO_BASE64'",
  "audio_format": "opus",
  "task": "translate",
  "target_languages": ["en", "ja"]
}'

API_URL="${API_URL:-http://localhost:8080/api/v1/process_audio}"
API_KEY="${API_KEY:-lingualink-demo-key}"

echo -e "${YELLOW}API URL: $API_URL${NC}"
echo -e "${YELLOW}测试文件: test/test.opus${NC}"
echo ""

# 单次测试函数
test_once() {
    local start=$(date +%s.%N)
    
    local response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $API_KEY" \
        -d "$REQUEST" 2>/dev/null)
    
    local end=$(date +%s.%N)
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | sed '$d')
    
    local latency=$(echo "$end - $start" | bc)
    local proc_time=$(echo "$body" | jq -r '.processing_time // 0' 2>/dev/null)
    local status=$(echo "$body" | jq -r '.status // "error"' 2>/dev/null)
    local transcription=$(echo "$body" | jq -r '.transcription // ""' 2>/dev/null | head -c 40)
    local corrected=$(echo "$body" | jq -r '.corrected_text // ""' 2>/dev/null | head -c 40)
    
    echo "HTTP: $http_code | 状态: $status | 延迟: ${latency}s | 处理: ${proc_time}s"
    echo "转录: $transcription"
    echo "纠正: $corrected"
    echo "翻译: $(echo "$body" | jq -r '.translations // {}' 2>/dev/null)"
}

# 热身请求
echo -e "${YELLOW}发送热身请求...${NC}"
curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d "$REQUEST" > /dev/null 2>&1 || true
sleep 1

# 测试
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  测试结果${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

for i in 1 2 3; do
    echo -e "${BLUE}--- 测试 #$i ---${NC}"
    test_once
    echo ""
    sleep 0.3
done

echo -e "${GREEN}完成！${NC}"
