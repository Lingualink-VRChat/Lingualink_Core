#!/bin/bash

# Lingualink Core 开发辅助脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 显示帮助
show_help() {
    echo "Lingualink Core 开发辅助脚本"
    echo
    echo "用法: $0 <命令> [选项]"
    echo
    echo "命令:"
    echo "  start          启动开发服务器"
    echo "  build          构建应用"
    echo "  test           运行测试"
    echo "  test-api       运行API测试"
    echo "  test-audio     运行音频处理测试"
    echo "  clean          清理构建文件"
    echo "  deps           更新依赖"
    echo "  format         格式化代码"
    echo "  lint           代码检查"
    echo "  docker         Docker相关操作"
    echo
    echo "Docker子命令:"
    echo "  docker build   构建Docker镜像"
    echo "  docker run     运行Docker容器"
    echo "  docker stop    停止Docker容器"
    echo
    echo "示例:"
    echo "  $0 start                # 启动开发服务器"
    echo "  $0 test                 # 运行所有测试"
    echo "  $0 docker build         # 构建Docker镜像"
}

# 启动开发服务器
start_dev() {
    log_info "启动开发服务器..."
    ./start.sh --dev
}

# 构建应用
build_app() {
    log_info "构建应用..."
    mkdir -p bin
    
    # 构建服务器
    go build -o bin/lingualink-server cmd/server/main.go
    log_success "服务器构建完成: bin/lingualink-server"
    
    # 如果有CLI，也构建CLI
    if [[ -f "cmd/cli/main.go" ]]; then
        go build -o bin/lingualink-cli cmd/cli/main.go
        log_success "CLI构建完成: bin/lingualink-cli"
    fi
}

# 运行测试
run_tests() {
    log_info "运行Go单元测试..."
    if go test ./...; then
        log_success "单元测试通过"
    else
        log_error "单元测试失败"
        return 1
    fi
    
    log_info "运行API集成测试..."
    if ./quick_test.sh; then
        log_success "API测试通过"
    else
        log_warning "API测试失败（可能服务未启动）"
    fi
}

# 运行API测试
run_api_tests() {
    log_info "运行完整API测试..."
    ./test_api.sh
}

# 运行音频测试
run_audio_tests() {
    log_info "运行音频处理测试..."
    ./test_audio.sh
}

# 清理构建文件
clean_build() {
    log_info "清理构建文件..."
    rm -rf bin/
    rm -rf dist/
    rm -rf *.log
    log_success "清理完成"
}

# 更新依赖
update_deps() {
    log_info "更新Go依赖..."
    go mod tidy
    go mod download
    log_success "依赖更新完成"
}

# 格式化代码
format_code() {
    log_info "格式化Go代码..."
    go fmt ./...
    log_success "代码格式化完成"
}

# 代码检查
lint_code() {
    log_info "运行代码检查..."
    
    # go vet
    if go vet ./...; then
        log_success "go vet 检查通过"
    else
        log_error "go vet 检查失败"
        return 1
    fi
    
    # 如果安装了golangci-lint
    if command -v golangci-lint &> /dev/null; then
        if golangci-lint run; then
            log_success "golangci-lint 检查通过"
        else
            log_error "golangci-lint 检查失败"
            return 1
        fi
    else
        log_warning "golangci-lint 未安装，跳过高级检查"
    fi
}

# Docker操作
docker_build() {
    log_info "构建Docker镜像..."
    docker build -t lingualink-core:latest .
    log_success "Docker镜像构建完成"
}

docker_run() {
    log_info "运行Docker容器..."
    docker run -d \
        --name lingualink-core \
        -p 8080:8080 \
        -v $(pwd)/config:/app/config \
        lingualink-core:latest
    log_success "Docker容器已启动"
}

docker_stop() {
    log_info "停止Docker容器..."
    docker stop lingualink-core || true
    docker rm lingualink-core || true
    log_success "Docker容器已停止"
}

# 主函数
main() {
    case "${1:-}" in
        start)
            start_dev
            ;;
        build)
            build_app
            ;;
        test)
            run_tests
            ;;
        test-api)
            run_api_tests
            ;;
        test-audio)
            run_audio_tests
            ;;
        clean)
            clean_build
            ;;
        deps)
            update_deps
            ;;
        format)
            format_code
            ;;
        lint)
            lint_code
            ;;
        docker)
            case "${2:-}" in
                build)
                    docker_build
                    ;;
                run)
                    docker_run
                    ;;
                stop)
                    docker_stop
                    ;;
                *)
                    log_error "未知的Docker命令: ${2:-}"
                    show_help
                    exit 1
                    ;;
            esac
            ;;
        --help|-h|help|"")
            show_help
            ;;
        *)
            log_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

main "$@" 