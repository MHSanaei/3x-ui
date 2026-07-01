#!/bin/bash

# Остановка скрипта при любой критической ошибке
set -e

# 1. Автоопределение архитектуры по умолчанию
DEFAULT_ARCH=$(uname -m)
case "$DEFAULT_ARCH" in
    x86_64)  CURRENT_ARCH="amd64" ;;
    aarch64) CURRENT_ARCH="arm64" ;;
    *)       CURRENT_ARCH="amd64" ;;
esac

# Цель сборки: "all", "windows", "linux", или конкретная архитектура вроде "amd64"
TARGET=${1:-$CURRENT_ARCH}

echo "=========================================="
echo "🚀 Запуск мультиплатформенной сборки. Цель: $TARGET"
echo "=========================================="

# Определение структуры проекта (корень репозитория или внешняя папка)
if [ -d "3x-ui" ]; then
    BASE_DIR="$(pwd)/3x-ui"
elif [ -f "main.go" ] && [ -d "frontend" ]; then
    BASE_DIR="$(pwd)"
else
    echo "❌ Ошибка: Исходники 3x-ui не найдены ни в текущей папке, ни в подпапке '3x-ui'!"
    exit 1
fi

echo "📂 Путь к исходникам: $BASE_DIR"

# 2. Проверка и установка зависимостей (Включая кросс-компиляторы для Windows)
NEED_CROSS_ARM64=0
if [ "$TARGET" = "all" ] || [ "$TARGET" = "linux" ] || [ "$TARGET" = "arm64" ]; then
    if [ "$CURRENT_ARCH" = "amd64" ]; then NEED_CROSS_ARM64=1; fi
fi

NEED_CROSS_WIN=0
if [ "$TARGET" = "all" ] || [ "$TARGET" = "windows" ]; then
    NEED_CROSS_WIN=1
fi

NODE_TOO_OLD=0
if command -v node &> /dev/null; then
    NODE_VER=$(node -v 2>/dev/null | cut -d'v' -f2 | cut -d'.' -f1)
    if [ "$NODE_VER" -lt 20 ]; then NODE_TOO_OLD=1; fi
else
    NODE_TOO_OLD=1
fi

# Проверяем, чего не хватает
MISSING_DEPS=0
if ! command -v npm &> /dev/null; then MISSING_DEPS=1; fi
if ! command -v go &> /dev/null; then MISSING_DEPS=1; fi
if [ "$NODE_TOO_OLD" -eq 1 ]; then MISSING_DEPS=1; fi
if [ "$NEED_CROSS_ARM64" -eq 1 ] && ! command -v aarch64-linux-gnu-gcc &> /dev/null; then MISSING_DEPS=1; fi
if [ "$NEED_CROSS_WIN" -eq 1 ] && ! command -v x86_64-w64-mingw32-gcc &> /dev/null; then MISSING_DEPS=1; fi

if [ "$MISSING_DEPS" -eq 1 ]; then
    if [ "$EUID" -ne 0 ]; then
        echo "❌ Компоненты отсутствуют. Запустите через sudo: sudo bash $0 $TARGET"
        exit 1
    fi
    
    apt-get update && apt-get install -y curl zip unzip 2>/dev/null || apt-get install -y curl
    
    # Установка Node.js 22
    if [ "$NODE_TOO_OLD" -eq 1 ]; then
        apt-get remove -y nodejs npm libnode-dev 2>/dev/null || true
        curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && apt-get install -y nodejs
    fi
    
    # Установка Go
    if ! command -v go &> /dev/null; then apt-get install -y golang-go; fi

    # Установка кросс-компилятора Си под ARM64
    if [ "$NEED_CROSS_ARM64" -eq 1 ] && ! command -v aarch64-linux-gnu-gcc &> /dev/null; then
        echo "🛠 Установка Си-компилятора для сборки под ARM64..."
        apt-get install -y gcc-aarch64-linux-gnu
    fi

    # Установка кросс-компилятора Си под Windows (Mingw-w64)
    if [ "$NEED_CROSS_WIN" -eq 1 ] && ! command -v x86_64-w64-mingw32-gcc &> /dev/null; then
        echo "🛠 Установка Си-компилятора для сборки под Windows (mingw-w64)..."
        apt-get install -y mingw-w64
    fi
fi

# 3. Создаем папку для билдов
BUILD_OUT_DIR="$(pwd)/build"
mkdir -p "$BUILD_OUT_DIR"

# 4. Сборка фронтенда
if [ -d "$BASE_DIR/frontend" ]; then
    echo "📦 Шаг 1: Сборка фронтенда панели..."
    pushd "$BASE_DIR/frontend" > /dev/null
    rm -rf node_modules package-lock.json 2>/dev/null
    npm install && npm run build
    popd > /dev/null
fi

# Функция компиляции
compile_target() {
    local os=$1
    local arch=$2
    local extension=""
    local env_cc=""
    
    if [ "$os" = "windows" ]; then
        extension=".exe"
        if [ "$arch" = "amd64" ]; then
            env_cc="CC=x86_64-w64-mingw32-gcc"
        elif [ "$arch" = "386" ]; then
            env_cc="CC=i686-w64-mingw32-gcc"
        fi
    fi

    if [ "$os" = "linux" ] && [ "$arch" = "arm64" ] && [ "$CURRENT_ARCH" = "amd64" ]; then
        env_cc="CC=aarch64-linux-gnu-gcc"
    fi

    echo "🐹 Шаг 2: Компиляция Go под [$os | $arch] с поддержкой CGO..."
    
    pushd "$BASE_DIR" > /dev/null
    go mod tidy
    
    env GOOS=$os GOARCH=$arch CGO_ENABLED=1 $env_cc go build -o "$BUILD_OUT_DIR/x-ui-$os-$arch$extension" main.go
    popd > /dev/null
    echo "✅ Создан файл: build/x-ui-$os-$arch$extension"
}

# 5. Логика выбора платформы
case "$TARGET" in
    all)
        echo "🌍 Сборка абсолютно под ВСЁ (Linux + Windows)..."
        compile_target "linux" "amd64"
        compile_target "linux" "arm64"
        compile_target "windows" "amd64"
        compile_target "windows" "386"
        ;;
    windows)
        echo "🪟 Сборка только под Windows..."
        compile_target "windows" "amd64"
        compile_target "windows" "386"
        ;;
    linux)
        echo "🐧 Сборка только под Linux..."
        compile_target "linux" "amd64"
        compile_target "linux" "arm64"
        ;;
    *)
        compile_target "linux" "$TARGET"
        ;;
esac

echo "=========================================="
echo "🏁 Сборка завершена! Содержимое папки 'build/':"
ls -lh "$BUILD_OUT_DIR"
echo "=========================================="
