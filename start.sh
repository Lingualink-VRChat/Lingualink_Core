#!/bin/bash

# Lingualink Core 启动脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
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

log_debug() {
    echo -e "${PURPLE}[DEBUG]${NC} $1"
}

log_param() {
    echo -e "${CYAN}[PARAM]${NC} $1"
}

# 显示当前环境变量和参数
show_runtime_info() {
    echo "=========================================="
    echo "         运行时信息 (Runtime Info)"
    echo "=========================================="
    
    log_param "当前目录: $(pwd)"
    log_param "脚本参数: $*"
    log_param "Go版本: $(go version)"
    
    echo
    log_info "环境变量 (Environment Variables):"
    log_param "LINGUALINK_CONFIG_DIR: ${LINGUALINK_CONFIG_DIR:-未设置}"
    log_param "VLLM_SERVER_URL: ${VLLM_SERVER_URL:-未设置(默认: http://localhost:8000/v1)}"
    log_param "MODEL_NAME: ${MODEL_NAME:-未设置(默认: qwen2.5-32b-instruct)}"
    log_param "API_KEY: ${API_KEY:+已设置}${API_KEY:-未设置}"
    log_param "LOG_LEVEL: ${LOG_LEVEL:-未设置(默认: info)}"
    log_param "AUTO_STOP_CONFLICTING: ${AUTO_STOP_CONFLICTING:-未设置(手动选择模式)}"
    
    echo
    log_info "Go模块信息:"
    if [[ -f "go.mod" ]]; then
        log_param "模块名: $(grep '^module' go.mod | awk '{print $2}')"
        log_param "Go版本要求: $(grep '^go' go.mod | awk '{print $2}')"
    fi
    
    echo
}

# 显示配置文件内容（排除敏感信息）
show_config_info() {
    if [[ -f "config/config.yaml" ]]; then
        log_info "配置文件预览 (排除敏感信息):"
        echo "----------------------------------------"
        # 显示配置但隐藏可能的敏感信息
        grep -v -E "(api_key|password|secret|token)" config/config.yaml | head -20 || true
        echo "----------------------------------------"
        echo
    fi
}

# 检查端口占用情况
check_port_usage() {
    local port="${1:-8080}"
    
    log_info "检查端口 $port 占用情况..."
    
    # 检查端口是否被占用
    local port_info=$(lsof -i :$port 2>/dev/null || true)
    if [[ -n "$port_info" ]]; then
        log_warning "端口 $port 被占用:"
        echo "$port_info" | head -5
        echo
        
        # 检查是否是Lingualink相关进程
        local lingualink_processes=$(echo "$port_info" | grep -E "(lingualink|main\.go)" || true)
        if [[ -n "$lingualink_processes" ]]; then
            log_warning "发现可能是之前的Lingualink服务占用端口"
            
            # 询问是否自动停止
            if [[ "${AUTO_STOP_CONFLICTING:-}" == "true" ]]; then
                log_info "AUTO_STOP_CONFLICTING=true，自动停止冲突进程..."
                if [[ -f "./stop.sh" ]]; then
                    ./stop.sh --port $port --force
                else
                    log_warning "未找到 stop.sh 脚本，手动停止进程..."
                    local pids=$(lsof -t -i :$port 2>/dev/null || true)
                    if [[ -n "$pids" ]]; then
                        echo "$pids" | while read pid; do
                            if [[ -n "$pid" ]]; then
                                log_debug "强制停止进程: $pid"
                                kill -9 "$pid" 2>/dev/null || true
                            fi
                        done
                        sleep 1
                    fi
                fi
            else
                echo
                log_warning "检测到端口冲突！请选择处理方式："
                echo "1) 自动停止冲突的进程"
                echo "2) 手动处理后重新启动"
                echo "3) 退出脚本"
                read -p "请选择 (1/2/3): " -n 1 -r
                echo
                
                case $REPLY in
                    1)
                        log_info "自动停止冲突进程..."
                        if [[ -f "./stop.sh" ]]; then
                            ./stop.sh --port $port
                        else
                            log_warning "未找到 stop.sh 脚本，手动停止进程..."
                            local pids=$(lsof -t -i :$port 2>/dev/null || true)
                            if [[ -n "$pids" ]]; then
                                echo "$pids" | while read pid; do
                                    if [[ -n "$pid" ]]; then
                                        log_debug "停止进程: $pid"
                                        kill -TERM "$pid" 2>/dev/null || true
                                    fi
                                done
                                sleep 2
                                # 再次检查
                                local remaining_pids=$(lsof -t -i :$port 2>/dev/null || true)
                                if [[ -n "$remaining_pids" ]]; then
                                    log_warning "部分进程未响应，强制停止..."
                                    echo "$remaining_pids" | while read pid; do
                                        if [[ -n "$pid" ]]; then
                                            kill -9 "$pid" 2>/dev/null || true
                                        fi
                                    done
                                fi
                            fi
                        fi
                        ;;
                    2)
                        log_info "请手动处理端口冲突后重新运行脚本"
                        log_info "可以使用: ./stop.sh --port $port"
                        exit 1
                        ;;
                    3)
                        log_info "退出脚本"
                        exit 0
                        ;;
                    *)
                        log_error "无效选择"
                        exit 1
                        ;;
                esac
            fi
        else
            log_error "端口 $port 被其他非Lingualink进程占用"
            log_info "请手动处理端口冲突或使用其他端口"
            log_info "可以使用: ./stop.sh --port $port 来释放端口"
            exit 1
        fi
        
        # 再次检查端口是否释放
        sleep 1
        local final_port_info=$(lsof -i :$port 2>/dev/null || true)
        if [[ -n "$final_port_info" ]]; then
            log_error "端口 $port 仍然被占用，无法启动服务"
            exit 1
        fi
    fi
    
    log_success "端口 $port 可用"
}

# 检查Go环境
check_go() {
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go 1.21 or later."
        exit 1
    fi

    go_version=$(go version | awk '{print $3}' | sed 's/go//')
    log_info "Go version: $go_version"
}

# 检查配置文件
check_config() {
    if [[ ! -f "config/config.yaml" ]]; then
        log_error "Configuration file not found: config/config.yaml"
        log_info "Please create the configuration file or copy from config/config.example.yaml"
        exit 1
    fi
    log_success "Configuration file found"
}

# 检查依赖
check_dependencies() {
    log_info "Checking dependencies..."
    if ! go mod verify &> /dev/null; then
        log_warning "Dependencies verification failed, running go mod tidy..."
        go mod tidy
    fi
    log_success "Dependencies verified"
}

# 构建应用
build_app() {
    log_info "Building application..."
    if go build -o bin/lingualink-server cmd/server/main.go; then
        log_success "Build completed successfully"
        
        # 显示构建产物信息
        if [[ -f "bin/lingualink-server" ]]; then
            file_size=$(ls -lh bin/lingualink-server | awk '{print $5}')
            log_param "Binary size: $file_size"
            log_param "Binary path: bin/lingualink-server"
        fi
    else
        log_error "Build failed"
        exit 1
    fi
}

# 启动服务（带输出捕获和分析）
start_server() {
    local build_mode="$1"
    local verbose_mode="$2"
    
    log_info "Starting Lingualink Core server..."
    
    # 设置环境变量
    export LINGUALINK_CONFIG_DIR="./config"
    
    # 显示启动信息
    echo "=========================================="
    echo "         服务启动信息"
    echo "=========================================="
    
    if [[ "$build_mode" == "--build" ]]; then
        log_param "启动方式: 构建模式 (使用编译后的二进制)"
        log_param "可执行文件: ./bin/lingualink-server"
    else
        log_param "启动方式: 开发模式 (直接运行Go代码)"
        log_param "入口文件: cmd/server/main.go"
    fi
    
    log_param "配置目录: $LINGUALINK_CONFIG_DIR"
    log_param "启动时间: $(date)"
    
    echo
    log_info "服务输出 (Server Output):"
    echo "=========================================="
    
    # 创建日志文件
    local log_file="logs/server_$(date +%Y%m%d_%H%M%S).log"
    mkdir -p logs
    
    # 启动服务器并捕获输出
    if [[ "$build_mode" == "--build" ]]; then
        if [[ "$verbose_mode" == "--verbose" ]]; then
            # 详细模式：同时输出到终端和日志文件
            ./bin/lingualink-server 2>&1 | tee "$log_file"
        else
            # 正常模式：只输出到终端，但也保存日志
            ./bin/lingualink-server 2>&1 | tee "$log_file"
        fi
    else
        if [[ "$verbose_mode" == "--verbose" ]]; then
            # 详细模式：显示Go编译过程
            log_debug "Go run with verbose output..."
            go run -x cmd/server/main.go 2>&1 | tee "$log_file"
        else
            go run cmd/server/main.go 2>&1 | tee "$log_file"
        fi
    fi
}

# 显示帮助信息
show_help() {
    echo "Lingualink Core 启动脚本"
    echo
    echo "用法: $0 [选项]"
    echo
    echo "选项:"
    echo "  --build        构建后运行（推荐用于生产环境）"
    echo "  --dev          开发模式运行（默认）"
    echo "  --verbose      详细输出模式（显示更多调试信息）"
    echo "  --check        仅检查环境，不启动服务"
    echo "  --info         显示运行时信息"
    echo "  --help, -h     显示帮助信息"
    echo
    echo "组合使用:"
    echo "  --dev --verbose    开发模式 + 详细输出"
    echo "  --build --verbose  构建模式 + 详细输出"
    echo
    echo "环境变量:"
    echo "  VLLM_SERVER_URL        VLLM服务器地址（默认: http://localhost:8000/v1）"
    echo "  MODEL_NAME             模型名称（默认: qwen2.5-32b-instruct）"
    echo "  API_KEY               API密钥"
    echo "  LOG_LEVEL             日志级别（debug/info/warn/error）"
    echo "  AUTO_STOP_CONFLICTING  自动停止冲突进程（true/false，默认: false）"
    echo
    echo "端口冲突处理:"
    echo "  - 启动前会自动检查端口8080占用情况"
    echo "  - 如果发现Lingualink相关进程占用，可选择自动停止"
    echo "  - 设置 AUTO_STOP_CONFLICTING=true 可自动停止冲突进程"
    echo "  - 可使用 ./stop.sh 手动管理服务进程"
    echo
    echo "输出说明:"
    echo "  - 服务启动参数和环境信息会在启动时显示"
    echo "  - 服务输出会同时显示在终端并保存到 logs/ 目录"
    echo "  - 使用 --verbose 可以看到更详细的调试信息"
    echo
    echo "示例:"
    echo "  $0                               # 开发模式启动"
    echo "  $0 --build                       # 构建后启动"
    echo "  $0 --dev --verbose               # 开发模式 + 详细输出"
    echo "  AUTO_STOP_CONFLICTING=true $0    # 自动停止冲突进程并启动"
    echo "  $0 --check                       # 仅检查环境"
    echo "  $0 --info                        # 显示运行时信息"
}

# 主函数
main() {
    echo "=========================================="
    echo "       Lingualink Core 启动脚本"
    echo "=========================================="
    
    local build_mode=""
    local verbose_mode=""
    local check_only=""
    local info_only=""
    
    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_help
                exit 0
                ;;
            --check)
                check_only="true"
                ;;
            --info)
                info_only="true"
                ;;
            --build)
                build_mode="--build"
                ;;
            --dev)
                build_mode=""
                ;;
            --verbose)
                verbose_mode="--verbose"
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
        shift
    done
    
    # 显示运行时信息
    if [[ "$info_only" == "true" ]]; then
        show_runtime_info
        show_config_info
        exit 0
    fi
    
    # 环境检查
    if [[ "$check_only" == "true" ]]; then
        log_info "环境检查模式"
        show_runtime_info
        check_go
        check_config
        check_dependencies
        show_config_info
        log_success "✅ 环境检查完成，所有检查通过"
        exit 0
    fi
    
    # 显示运行时信息
    show_runtime_info
    
    # 环境检查
    check_go
    check_config
    check_dependencies
    
    # 显示配置信息
    if [[ "$verbose_mode" == "--verbose" ]]; then
        show_config_info
    fi
    
    # 检查端口占用情况（在启动前）
    check_port_usage 8080
    
    if [[ "$build_mode" == "--build" ]]; then
        log_info "构建模式启动"
        
        # 创建bin目录
        mkdir -p bin
        
        build_app
        start_server --build "$verbose_mode"
    else
        log_info "开发模式启动"
        start_server "" "$verbose_mode"
    fi
}

# 信号处理
cleanup() {
    echo
    log_info "正在关闭服务..."
    log_info "日志文件保存在 logs/ 目录中"
    # 这里可以添加清理逻辑
    exit 0
}

trap cleanup SIGINT SIGTERM

# 运行主函数
main "$@" 