#!/bin/bash
# 性能测试脚本（单模式：Tool Use Pipeline）

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# 配置
TEST_AUDIO="$PROJECT_DIR/test/test.opus"
API_URL="${API_URL:-http://localhost:8080/api/v1/process_audio}"
API_KEY="${API_KEY:-lingualink-demo-key}"
ITERATIONS="${ITERATIONS:-3}"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}    Lingualink 性能测试${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查测试文件
if [ ! -f "$TEST_AUDIO" ]; then
  echo -e "${RED}错误: 测试文件 $TEST_AUDIO 不存在${NC}"
  exit 1
fi

# Base64 编码音频
echo -e "${YELLOW}正在编码测试音频...${NC}"
AUDIO_BASE64=$(base64 -w 0 "$TEST_AUDIO")

# 构建请求 JSON
REQUEST_JSON=$(cat <<EOF
{
  "audio": "$AUDIO_BASE64",
  "audio_format": "opus",
  "task": "translate",
  "target_languages": ["en", "ja"]
}
EOF
)

run_test() {
  local start_time
  start_time=$(date +%s.%N)
  local response
  response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d "$REQUEST_JSON")
  local end_time
  end_time=$(date +%s.%N)

  local http_code
  http_code=$(echo "$response" | tail -n 1)
  local body
  body=$(echo "$response" | sed '$d')

  local latency
  latency=$(echo "$end_time - $start_time" | bc)
  local processing_time
  processing_time=$(echo "$body" | jq -r '.processing_time // 0' 2>/dev/null || echo 0)
  local transcription
  transcription=$(echo "$body" | jq -r '.transcription // "N/A"' 2>/dev/null | head -c 50)

  echo "$latency,$processing_time,$http_code,$transcription"
}

RESULTS=()
echo ""
echo "迭代 | 总延迟(s) | 处理时间(s) | HTTP状态 | 转录前50字"
echo "-----+------------+-------------+---------+------------------"

for i in $(seq 1 "$ITERATIONS"); do
  result=$(run_test)
  RESULTS+=("$result")

  latency=$(echo "$result" | cut -d',' -f1)
  proc_time=$(echo "$result" | cut -d',' -f2)
  http_code=$(echo "$result" | cut -d',' -f3)
  transcription=$(echo "$result" | cut -d',' -f4-)

  printf "  %d  | %10.3f | %11.3f | %7s | %s\n" \
    "$i" "$latency" "$proc_time" "$http_code" "$transcription"

  sleep 0.5
done

AVG_LATENCY=$(printf '%s\n' "${RESULTS[@]}" | cut -d',' -f1 | awk '{sum+=$1} END {print sum/NR}')
AVG_PROC=$(printf '%s\n' "${RESULTS[@]}" | cut -d',' -f2 | awk '{sum+=$1} END {print sum/NR}')

echo ""
echo -e "${GREEN}平均延迟: ${AVG_LATENCY}s${NC}"
echo -e "${GREEN}平均处理时间: ${AVG_PROC}s${NC}"
echo ""
echo -e "${GREEN}测试完成！${NC}"
