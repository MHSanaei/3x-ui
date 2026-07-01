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

# 2. Проверка и установка зависимостей (Node.js v22 и Go)
if ! command -v npm &> /dev/null || ! command -v go &> /dev/null || [[ $(node -v 2>/dev/null | cut -d'v' -f2 | cut -d'.' -f1) -lt 20 ]]; then
    if [ "$EUID" -ne 0 ]; then
        echo "❌ Компоненты отсутствуют. Запустите через sudo: sudo bash $0 $TARGET"
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

# 3. Создаем папку для билдов (всегда в текущем рабочем каталоге запуска)
BUILD_OUT_DIR="$(pwd)/build"
mkdir -p "$BUILD_OUT_DIR"

# 4. Сборка фронтенда (один раз для всех платформ)
if [ -d "$BASE_DIR/frontend" ]; then
    echo "📦 Шаг 1: Сборка фронтенда панели..."
    pushd "$BASE_DIR/frontend" > /dev/null
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
    
    pushd "$BASE_DIR" > /dev/null
    
    echo "🔄 Подмена драйвера SQLite на Pure-Go версию (без CGO)..."
    # Находим все упоминания старого драйвера во всех .go файлах проекта и заменяем на modernc.org/sqlite
    find . -type f -name "*.go" -exec sed -i 's|"github.com/mattn/go-sqlite3"|"modernc.org/sqlite"|g' {} +
    
    # Загружаем новый драйвер в кэш модулей Go перед сборкой
    go get modernc.org/sqlite@latest 2>/dev/null || true
    go mod tidy
    
    # Кросс-компиляция силами самого Go с отключенным CGO (теперь это полностью автономный бинарник)
    env GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -o "$BUILD_OUT_DIR/x-ui-$os-$arch$extension" main.go
    popd > /dev/null
    echo "✅ Создан файл: build/x-ui-$os-$arch$extension"
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
ls -lh "$BUILD_OUT_DIR"
echo "=========================================="
