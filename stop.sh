#!/bin/bash

# Lingualink Core 停止脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 默认配置
DEFAULT_PORT=8080
DEFAULT_PROCESS_NAME="lingualink-server"
DEFAULT_GO_PROCESS="main.go"

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

log_debug() {
    echo -e "${PURPLE}[DEBUG]${NC} $1"
}

log_param() {
    echo -e "${CYAN}[PARAM]${NC} $1"
}

# 显示当前运行状态
show_status() {
    echo "=========================================="
    echo "         Lingualink Core 服务状态"
    echo "=========================================="
    
    log_info "检查服务运行状态..."
    
    # 检查二进制进程
    local binary_pids=$(pgrep -f "$DEFAULT_PROCESS_NAME" 2>/dev/null || true)
    if [[ -n "$binary_pids" ]]; then
        log_warning "发现运行中的二进制服务进程:"
        echo "$binary_pids" | while read pid; do
            if [[ -n "$pid" ]]; then
                local cmd=$(ps -p "$pid" -o command= 2>/dev/null || echo "Unknown")
                log_param "PID: $pid, Command: $cmd"
            fi
        done
    else
        log_success "未发现运行中的二进制服务进程"
    fi
    
    # 检查Go运行进程
    local go_pids=$(pgrep -f "$DEFAULT_GO_PROCESS" 2>/dev/null || true)
    if [[ -n "$go_pids" ]]; then
        log_warning "发现运行中的Go开发进程:"
        echo "$go_pids" | while read pid; do
            if [[ -n "$pid" ]]; then
                local cmd=$(ps -p "$pid" -o command= 2>/dev/null || echo "Unknown")
                log_param "PID: $pid, Command: $cmd"
            fi
        done
    else
        log_success "未发现运行中的Go开发进程"
    fi
    
    # 检查端口占用
    log_info "检查端口 $DEFAULT_PORT 占用情况..."
    local port_info=$(lsof -i :$DEFAULT_PORT 2>/dev/null || true)
    if [[ -n "$port_info" ]]; then
        log_warning "端口 $DEFAULT_PORT 被占用:"
        echo "$port_info" | head -10
    else
        log_success "端口 $DEFAULT_PORT 未被占用"
    fi
    
    echo
}

# 通过进程名停止服务
stop_by_process_name() {
    local process_name="$1"
    local force_mode="$2"
    
    log_info "查找进程: $process_name"
    local pids=$(pgrep -f "$process_name" 2>/dev/null || true)
    
    if [[ -z "$pids" ]]; then
        log_success "未发现 $process_name 相关进程"
        return 0
    fi
    
    log_warning "发现进程: $pids"
    
    if [[ "$force_mode" == "--force" ]]; then
        log_warning "强制终止进程..."
        echo "$pids" | while read pid; do
            if [[ -n "$pid" ]]; then
                log_debug "SIGKILL: $pid"
                kill -9 "$pid" 2>/dev/null || true
            fi
        done
    else
        log_info "优雅停止进程..."
        echo "$pids" | while read pid; do
            if [[ -n "$pid" ]]; then
                log_debug "SIGTERM: $pid"
                kill -TERM "$pid" 2>/dev/null || true
            fi
        done
        
        # 等待进程退出
        sleep 2
        
        # 检查是否还有残留进程
        local remaining_pids=$(pgrep -f "$process_name" 2>/dev/null || true)
        if [[ -n "$remaining_pids" ]]; then
            log_warning "部分进程未响应SIGTERM，发送SIGKILL..."
            echo "$remaining_pids" | while read pid; do
                if [[ -n "$pid" ]]; then
                    log_debug "SIGKILL: $pid"
                    kill -9 "$pid" 2>/dev/null || true
                fi
            done
        fi
    fi
    
    # 再次检查
    sleep 1
    local final_pids=$(pgrep -f "$process_name" 2>/dev/null || true)
    if [[ -z "$final_pids" ]]; then
        log_success "成功停止 $process_name 进程"
    else
        log_error "停止 $process_name 进程失败，仍有进程运行: $final_pids"
        return 1
    fi
}

# 通过端口停止服务
stop_by_port() {
    local port="$1"
    local force_mode="$2"
    
    log_info "查找占用端口 $port 的进程..."
    
    # 获取占用端口的进程信息
    local port_processes=$(lsof -t -i :$port 2>/dev/null || true)
    
    if [[ -z "$port_processes" ]]; then
        log_success "端口 $port 未被占用"
        return 0
    fi
    
    log_warning "发现占用端口 $port 的进程: $port_processes"
    
    # 显示详细信息
    lsof -i :$port 2>/dev/null | head -10 || true
    
    if [[ "$force_mode" == "--force" ]]; then
        log_warning "强制终止占用端口的进程..."
        echo "$port_processes" | while read pid; do
            if [[ -n "$pid" ]]; then
                log_debug "SIGKILL: $pid"
                kill -9 "$pid" 2>/dev/null || true
            fi
        done
    else
        log_info "优雅停止占用端口的进程..."
        echo "$port_processes" | while read pid; do
            if [[ -n "$pid" ]]; then
                log_debug "SIGTERM: $pid"
                kill -TERM "$pid" 2>/dev/null || true
            fi
        done
        
        # 等待进程退出
        sleep 2
        
        # 检查端口是否释放
        local remaining_processes=$(lsof -t -i :$port 2>/dev/null || true)
        if [[ -n "$remaining_processes" ]]; then
            log_warning "部分进程未响应SIGTERM，发送SIGKILL..."
            echo "$remaining_processes" | while read pid; do
                if [[ -n "$pid" ]]; then
                    log_debug "SIGKILL: $pid"
                    kill -9 "$pid" 2>/dev/null || true
                fi
            done
        fi
    fi
    
    # 再次检查端口
    sleep 1
    local final_processes=$(lsof -t -i :$port 2>/dev/null || true)
    if [[ -z "$final_processes" ]]; then
        log_success "成功释放端口 $port"
    else
        log_error "释放端口 $port 失败，仍有进程占用: $final_processes"
        return 1
    fi
}

# 清理临时文件
cleanup_files() {
    log_info "清理临时文件..."
    
    # 清理可能的PID文件
    if [[ -f "lingualink.pid" ]]; then
        log_debug "移除 PID 文件: lingualink.pid"
        rm -f "lingualink.pid"
    fi
    
    # 清理临时日志文件（可选）
    if [[ -d "logs" ]]; then
        local log_count=$(find logs -name "*.log" -mtime +7 | wc -l)
        if [[ $log_count -gt 0 ]]; then
            log_info "发现 $log_count 个超过7天的日志文件"
            read -p "是否删除这些旧日志文件? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                find logs -name "*.log" -mtime +7 -delete
                log_success "已清理旧日志文件"
            fi
        fi
    fi
    
    log_success "临时文件清理完成"
}

# 显示帮助信息
show_help() {
    echo "Lingualink Core 停止脚本"
    echo
    echo "用法: $0 [选项]"
    echo
    echo "选项:"
    echo "  --status       显示当前运行状态"
    echo "  --stop         停止所有Lingualink服务"
    echo "  --port PORT    停止占用指定端口的进程（默认: $DEFAULT_PORT）"
    echo "  --force        强制停止（使用SIGKILL）"
    echo "  --cleanup      清理临时文件"
    echo "  --all          停止服务 + 清理文件"
    echo "  --help, -h     显示帮助信息"
    echo
    echo "组合使用:"
    echo "  --stop --force        强制停止所有服务"
    echo "  --port 8080 --force   强制释放8080端口"
    echo "  --all --force         强制停止并清理"
    echo
    echo "示例:"
    echo "  $0 --status              # 查看运行状态"
    echo "  $0 --stop                # 优雅停止服务"
    echo "  $0 --stop --force        # 强制停止服务"
    echo "  $0 --port 8080           # 释放8080端口"
    echo "  $0 --all                 # 停止服务并清理"
    echo
    echo "注意事项:"
    echo "  - 优雅停止会先发送SIGTERM，等待2秒后再发送SIGKILL"
    echo "  - 强制停止直接发送SIGKILL，可能导致数据丢失"
    echo "  - 建议先尝试优雅停止，失败后再使用强制停止"
}

# 主函数
main() {
    echo "=========================================="
    echo "       Lingualink Core 停止脚本"
    echo "=========================================="
    
    local action=""
    local force_mode=""
    local custom_port=""
    local show_status_only=""
    local cleanup_only=""
    local stop_all=""
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_help
                exit 0
                ;;
            --status)
                show_status_only="true"
                ;;
            --stop)
                action="stop"
                ;;
            --port)
                action="port"
                shift
                custom_port="$1"
                ;;
            --force)
                force_mode="--force"
                ;;
            --cleanup)
                cleanup_only="true"
                ;;
            --all)
                stop_all="true"
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
        shift
    done
    
    # 检查必要的命令
    if ! command -v lsof &> /dev/null; then
        log_error "lsof 命令未找到，请安装: brew install lsof (macOS) 或 apt-get install lsof (Linux)"
        exit 1
    fi
    
    # 显示状态
    if [[ "$show_status_only" == "true" ]]; then
        show_status
        exit 0
    fi
    
    # 仅清理
    if [[ "$cleanup_only" == "true" ]]; then
        cleanup_files
        exit 0
    fi
    
    # 停止所有服务并清理
    if [[ "$stop_all" == "true" ]]; then
        log_info "停止所有服务并清理..."
        show_status
        
        log_info "停止二进制服务进程..."
        stop_by_process_name "$DEFAULT_PROCESS_NAME" "$force_mode" || true
        
        log_info "停止Go开发进程..."
        stop_by_process_name "$DEFAULT_GO_PROCESS" "$force_mode" || true
        
        log_info "释放默认端口..."
        stop_by_port "$DEFAULT_PORT" "$force_mode" || true
        
        cleanup_files
        
        echo
        log_success "✅ 所有操作完成"
        show_status
        exit 0
    fi
    
    # 执行指定操作
    case "$action" in
        stop)
            log_info "停止Lingualink服务..."
            show_status
            
            stop_by_process_name "$DEFAULT_PROCESS_NAME" "$force_mode"
            stop_by_process_name "$DEFAULT_GO_PROCESS" "$force_mode"
            
            echo
            log_success "✅ 服务停止完成"
            show_status
            ;;
        port)
            local port="${custom_port:-$DEFAULT_PORT}"
            log_info "释放端口: $port"
            show_status
            
            stop_by_port "$port" "$force_mode"
            
            echo
            log_success "✅ 端口释放完成"
            show_status
            ;;
        "")
            log_info "未指定操作，显示当前状态"
            show_status
            echo
            log_info "使用 --help 查看可用操作"
            ;;
        *)
            log_error "未知操作: $action"
            show_help
            exit 1
            ;;
    esac
}

# 信号处理
cleanup_script() {
    echo
    log_info "脚本被中断"
    exit 0
}

trap cleanup_script SIGINT SIGTERM

# 运行主函数
main "$@" 