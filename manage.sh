#!/bin/bash
# Lingualink Core 服务管理脚本

set -e

# 配置
APP_NAME="lingualink-core"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY_NAME="server"
BINARY_PATH="$PROJECT_DIR/bin/$BINARY_NAME"
LOG_DIR="$PROJECT_DIR/logs"
PID_FILE="$LOG_DIR/server.pid"
LOG_FILE="$LOG_DIR/server.log"
CONFIG_PATH="${CONFIG_PATH:-$PROJECT_DIR/config/config.yaml}"
PORT_OVERRIDE="${PORT:-}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 确保日志目录存在
mkdir -p "$LOG_DIR"

detect_port_from_config() {
    if [ ! -f "$CONFIG_PATH" ]; then
        return 0
    fi
    awk '
        /^[[:space:]]*server:[[:space:]]*$/ { in_server=1; next }
        in_server && match($0, /^[[:space:]]*port:[[:space:]]*([0-9]+)/, m) { print m[1]; exit }
        in_server && /^[^[:space:]]/ { in_server=0 }
    ' "$CONFIG_PATH"
}

PORT="${PORT_OVERRIDE:-$(detect_port_from_config)}"
PORT="${PORT:-8080}"

# 运行时环境变量（让服务读取 CONFIG_PATH 对应配置）
CONFIG_DIR="$(dirname "$CONFIG_PATH")"

# 获取 PID
get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE"
    fi
}

# 检查是否运行中
is_running() {
    local pid=$(get_pid)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        return 0
    fi
    return 1
}

# 编译
build() {
    echo -e "${YELLOW}编译中...${NC}"
    mkdir -p "$PROJECT_DIR/bin"
    cd "$PROJECT_DIR"
    go build -o "$BINARY_PATH" ./cmd/server
    echo -e "${GREEN}编译完成: $BINARY_PATH${NC}"
}

# 启动
start() {
    if is_running; then
        echo -e "${YELLOW}服务已在运行中 (PID: $(get_pid))${NC}"
        return 1
    fi

    # 检查端口
    if lsof -i :$PORT -t >/dev/null 2>&1; then
        echo -e "${RED}端口 $PORT 已被占用${NC}"
        lsof -i :$PORT
        return 1
    fi

    # 编译（如果需要）
    if [ ! -f "$BINARY_PATH" ] || [ "$PROJECT_DIR/cmd/server/main.go" -nt "$BINARY_PATH" ]; then
        build
    fi

    echo -e "${YELLOW}启动服务...${NC}"
    nohup env \
        LINGUALINK_CONFIG_FILE="$CONFIG_PATH" \
        LINGUALINK_CONFIG_DIR="$CONFIG_DIR" \
        "$BINARY_PATH" >> "$LOG_FILE" 2>&1 &
    local pid=$!
    echo $pid > "$PID_FILE"
    
    sleep 2
    
    if is_running; then
        echo -e "${GREEN}服务已启动 (PID: $pid)${NC}"
        echo -e "日志: $LOG_FILE"
        echo -e "PID 文件: $PID_FILE"
    else
        echo -e "${RED}启动失败，请检查日志: $LOG_FILE${NC}"
        tail -20 "$LOG_FILE"
        return 1
    fi
}

# 停止
stop() {
    if ! is_running; then
        echo -e "${YELLOW}服务未在运行${NC}"
        rm -f "$PID_FILE"
        return 0
    fi

    local pid=$(get_pid)
    echo -e "${YELLOW}停止服务 (PID: $pid)...${NC}"
    
    # 优雅停止
    kill -TERM "$pid" 2>/dev/null
    
    # 等待退出
    local count=0
    while is_running && [ $count -lt 10 ]; do
        sleep 1
        count=$((count + 1))
    done
    
    # 强制停止
    if is_running; then
        echo -e "${YELLOW}强制停止...${NC}"
        kill -9 "$pid" 2>/dev/null
        sleep 1
    fi
    
    rm -f "$PID_FILE"
    echo -e "${GREEN}服务已停止${NC}"
}

# 重启
restart() {
    stop
    sleep 1
    start
}

# 状态
status() {
    if is_running; then
        local pid=$(get_pid)
        echo -e "${GREEN}服务运行中${NC}"
        echo "  PID: $pid"
        echo "  日志: $LOG_FILE"
        echo "  端口: $PORT"
        
        # 健康检查
        if curl -s "http://localhost:$PORT/api/v1/health" >/dev/null 2>&1; then
            echo -e "  健康: ${GREEN}正常${NC}"
        else
            echo -e "  健康: ${YELLOW}无响应${NC}"
        fi
    else
        echo -e "${RED}服务未运行${NC}"
        [ -f "$PID_FILE" ] && rm -f "$PID_FILE"
    fi
}

# 查看日志
logs() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo -e "${YELLOW}日志文件不存在: $LOG_FILE${NC}"
    fi
}

# 使用帮助
usage() {
    echo "Lingualink Core 服务管理脚本"
    echo ""
    echo "用法: $0 {start|stop|restart|status|logs|build}"
    echo ""
    echo "命令:"
    echo "  start    启动服务"
    echo "  stop     停止服务"
    echo "  restart  重启服务"
    echo "  status   查看状态"
    echo "  logs     查看日志"
    echo "  build    编译项目"
    echo ""
    echo "环境变量:"
    echo "  CONFIG_PATH  配置文件路径 (默认: config/config.yaml)"
    echo "  PORT         服务端口 (默认: 8080)"
}

# 主入口
case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    logs)
        logs
        ;;
    build)
        build
        ;;
    *)
        usage
        exit 1
        ;;
esac
