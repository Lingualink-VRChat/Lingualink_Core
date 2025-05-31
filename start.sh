#!/bin/bash

# Lingualink Core 启动脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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
    else
        log_error "Build failed"
        exit 1
    fi
}

# 启动服务
start_server() {
    log_info "Starting Lingualink Core server..."
    
    # 设置环境变量
    export LINGUALINK_CONFIG_DIR="./config"
    
    # 启动服务器
    if [[ "$1" == "--build" ]]; then
        ./bin/lingualink-server
    else
        go run cmd/server/main.go
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
    echo "  --check        仅检查环境，不启动服务"
    echo "  --help, -h     显示帮助信息"
    echo
    echo "环境变量:"
    echo "  VLLM_SERVER_URL    VLLM服务器地址（默认: http://localhost:8000/v1）"
    echo "  MODEL_NAME         模型名称（默认: qwen2.5-32b-instruct）"
    echo "  API_KEY           API密钥"
    echo "  LOG_LEVEL         日志级别（debug/info/warn/error）"
    echo
    echo "示例:"
    echo "  $0                 # 开发模式启动"
    echo "  $0 --build         # 构建后启动"
    echo "  $0 --check         # 仅检查环境"
}

# 主函数
main() {
    echo "=========================================="
    echo "       Lingualink Core 启动脚本"
    echo "=========================================="
    
    case "${1:-}" in
        --help|-h)
            show_help
            exit 0
            ;;
        --check)
            log_info "环境检查模式"
            check_go
            check_config
            check_dependencies
            log_success "✅ 环境检查完成，所有检查通过"
            exit 0
            ;;
        --build)
            log_info "构建模式启动"
            check_go
            check_config
            check_dependencies
            
            # 创建bin目录
            mkdir -p bin
            
            build_app
            start_server --build
            ;;
        --dev|"")
            log_info "开发模式启动"
            check_go
            check_config
            check_dependencies
            start_server
            ;;
        *)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
}

# 信号处理
cleanup() {
    log_info "正在关闭服务..."
    # 这里可以添加清理逻辑
    exit 0
}

trap cleanup SIGINT SIGTERM

# 运行主函数
main "$@" 