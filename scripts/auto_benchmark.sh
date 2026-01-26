#!/bin/bash
# 自动性能测试（单模式：Tool Use Pipeline）

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TEST_AUDIO="$PROJECT_DIR/test/test.opus"
ITERATIONS=3
API_URL="${API_URL:-http://localhost:8080/api/v1/process_audio}"
API_KEY="${API_KEY:-lingualink-demo-key}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Lingualink 性能测试${NC}"
echo -e "${BLUE}========================================${NC}"

# 检查文件
if [ ! -f "$TEST_AUDIO" ]; then
    echo -e "${RED}错误: $TEST_AUDIO 不存在${NC}"
    exit 1
fi

# 编码音频
AUDIO_BASE64=$(base64 -w 0 "$TEST_AUDIO")

# 请求函数
send_request() {
    curl -s -w '\n%{time_total}' \
        -H "X-API-Key: $API_KEY" \
        -H "Content-Type: application/json" \
        -X POST "$API_URL" \
        -d "{\"audio\": \"$AUDIO_BASE64\", \"audio_format\": \"opus\", \"task\": \"translate\", \"target_languages\": [\"en\", \"ja\"]}"
}

# 测试函数
run_tests() {
    local latencies=()
    
    echo -e "\n${YELLOW}测试 ($ITERATIONS 次)...${NC}\n"
    
    for i in $(seq 1 $ITERATIONS); do
        result=$(send_request)
        latency=$(echo "$result" | tail -1)
        body=$(echo "$result" | sed '$d')
        status=$(echo "$body" | jq -r '.status // "error"')
        proc_time=$(echo "$body" | jq -r '.processing_time // 0')
        
        latencies+=("$latency")
        printf "  #%d: 延迟=%.3fs 处理=%.3fs 状态=%s\n" "$i" "$latency" "$proc_time" "$status"
        sleep 0.5
    done
    
    # 计算平均值
    avg=$(printf '%s\n' "${latencies[@]}" | awk '{sum+=$1} END {printf "%.3f", sum/NR}')
    echo -e "\n  ${GREEN}平均延迟: ${avg}s${NC}"
}

run_tests

echo -e "\n${GREEN}测试完成！${NC}"
