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

# 2. Проверка и установка зависимостей (Node.js v22 и Go)
if ! command -v npm &> /dev/null || ! command -v go &> /dev/null || [[ $(node -v 2>/dev/null | cut -d'v' -f2 | cut -d'.' -f1) -lt 20 ]]; then
    if [ "$EUID" -ne 0 ]; then
        echo "❌ Компоненты отсутствуют. Запустите через sudo: sudo bash build.sh $TARGET"
        exit 1
    fi
    apt-get update && apt-get install -y curl zip unzip 2>/dev/null || apt-get install -y curl
    if ! command -v node &> /dev/null || [[ $(node -v 2>/dev/null | cut -d'v' -f2 | cut -d'.' -f1) -lt 20 ]]; then
        apt-get remove -y nodejs npm libnode-dev 2>/dev/null || true
        curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && apt-get install -y nodejs
    fi
    if ! command -v go &> /dev/null; then
        apt-get install -y golang-go
    fi
fi

# 3. Создаем папку для билдов
mkdir -p build

# 4. Сборка фронтенда (один раз для всех платформ)
if [ -d "3x-ui/frontend" ]; then
    echo "📦 Шаг 1: Сборка фронтенда панели..."
    pushd 3x-ui/frontend > /dev/null
    rm -rf node_modules package-lock.json 2>/dev/null
    npm install && npm run build
    popd > /dev/null
fi

# Функция компиляции: принимает ОС и Архитектуру
compile_target() {
    local os=$1
    local arch=$2
    local extension=""
    
    # Для Windows добавляем расширение .exe
    if [ "$os" = "windows" ]; then
        extension=".exe"
    fi

    echo "🐹 Шаг 2: Компиляция Go под [$os | $arch]..."
    if [ -d "3x-ui" ]; then
        pushd 3x-ui > /dev/null
        # Кросс-компиляция силами самого Go
        env GOOS=$os GOARCH=$arch go build -o ../build/x-ui-$os-$arch$extension main.go
        popd > /dev/null
        echo "✅ Создан файл: build/x-ui-$os-$arch$extension"
    else
        echo "❌ Исходники 3x-ui не найдены!"
        exit 1
    fi
}

# 5. Логика выбора платформы
case "$TARGET" in
    all)
        echo "🌍 Сборка абсолютно под ВСЁ (Linux + Windows)..."
        # Linux билды
        compile_target "linux" "amd64"
        compile_target "linux" "arm64"
        # Windows билды
        compile_target "windows" "amd64"
        compile_target "windows" "386"
        ;;
    windows)
        echo "🪟 Сборка только под Windows (amd64 и 32-bit)..."
        compile_target "windows" "amd64"
        compile_target "windows" "386"
        ;;
    linux)
        echo "🐧 Сборка только под Linux (amd64 и arm64)..."
        compile_target "linux" "amd64"
        compile_target "linux" "arm64"
        ;;
    *)
        # Если передана просто архитектура (например, amd64), собираем её под Linux
        compile_target "linux" "$TARGET"
        ;;
esac

echo "=========================================="
echo "🏁 Сборка завершена! Содержимое папки 'build/':"
ls -lh build/
echo "=========================================="

