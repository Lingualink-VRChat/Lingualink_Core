#!/bin/bash

# Lingualink Core API 测试脚本
API_KEY="lls-jm1Rg2Bt6HgCrDkzMou5Lu4t"
BASE_URL="http://localhost:8100/api/v1"
AUDIO_FILE="test/test.opus"

echo "=== Lingualink Core API 测试 ==="
echo ""

# 检查服务健康状态
echo "1. 检查服务健康状态..."
curl -s "$BASE_URL/health" | jq '.' 2>/dev/null || curl -s "$BASE_URL/health"
echo ""

# 检查音频文件
if [ ! -f "$AUDIO_FILE" ]; then
    echo "错误: 音频文件 $AUDIO_FILE 不存在"
    exit 1
fi

echo "2. 准备音频数据..."
AUDIO_BASE64=$(base64 -w 0 "$AUDIO_FILE")
echo "音频文件大小: $(wc -c < "$AUDIO_FILE") bytes"
echo ""

# 测试音频转录
echo "3. 测试音频转录..."
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "{
    \"audio\": \"$AUDIO_BASE64\",
    \"audio_format\": \"opus\",
    \"task\": \"transcribe\",
    \"source_language\": \"zh\",
    \"target_languages\": []
  }" \
  "$BASE_URL/process_audio" | jq '.' 2>/dev/null || echo "转录请求失败"
echo ""

# 测试音频翻译
echo "4. 测试音频翻译..."
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "{
    \"audio\": \"$AUDIO_BASE64\",
    \"audio_format\": \"opus\",
    \"task\": \"translate\",
    \"source_language\": \"zh\",
    \"target_languages\": [\"en\", \"ja\"]
  }" \
  "$BASE_URL/process_audio" | jq '.' 2>/dev/null || echo "翻译请求失败"
echo ""

# 测试文本翻译
echo "5. 测试文本翻译..."
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d "{
    \"text\": \"你好，世界！这是一个测试。\",
    \"target_languages\": [\"en\", \"ja\"]
  }" \
  "$BASE_URL/process_text" | jq '.' 2>/dev/null || echo "文本翻译请求失败"

echo ""
echo "测试完成！"
