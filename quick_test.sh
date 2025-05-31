#!/bin/bash

# Lingualink Core 快速测试脚本
# 快速验证基础功能

BASE_URL="http://localhost:8080"
API_KEY="dev-key-123"

echo "🚀 Lingualink Core 快速测试"
echo "================================"

# 1. 健康检查
echo -n "1. 健康检查... "
if curl -s -f -H "X-API-Key: $API_KEY" "$BASE_URL/api/v1/health" > /dev/null; then
    echo "✅ 通过"
else
    echo "❌ 失败"
    exit 1
fi

# 2. 获取能力信息
echo -n "2. 系统能力... "
response=$(curl -s -H "X-API-Key: $API_KEY" "$BASE_URL/api/v1/capabilities")
if echo "$response" | jq . > /dev/null 2>&1; then
    echo "✅ 通过"
    echo "   - 支持的任务: $(echo "$response" | jq -r '.tasks // [] | join(", ")')"
    echo "   - LLM后端: $(echo "$response" | jq -r '.backends // [] | join(", ")')"
else
    echo "❌ 失败"
fi

# 3. 语言列表
echo -n "3. 支持语言... "
response=$(curl -s -H "X-API-Key: $API_KEY" "$BASE_URL/api/v1/languages")
if echo "$response" | jq . > /dev/null 2>&1; then
    echo "✅ 通过"
    echo "   - 语言数量: $(echo "$response" | jq '. | length')"
else
    echo "❌ 失败"
fi

# 4. 监控指标
echo -n "4. 监控指标... "
if curl -s -f -H "X-API-Key: $API_KEY" "$BASE_URL/api/v1/admin/metrics" > /dev/null; then
    echo "✅ 通过"
else
    echo "❌ 失败"
fi

echo
echo "🎯 基础功能测试完成"
echo "如需测试音频处理功能，请运行: ./test_audio.sh" 