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

# Определение структуры проекта
if [ -d "3x-ui" ]; then
    BASE_DIR="$(pwd)/3x-ui"
elif [ -f "main.go" ] && [ -d "frontend" ]; then
    BASE_DIR="$(pwd)"
else
    echo "❌ Ошибка: Исходники 3x-ui не найдены ни в текущей папке, ни в подпапке '3x-ui'!"
    exit 1
fi

echo "📂 Путь к исходникам: $BASE_DIR"

# Функция определения менеджера пакетов
detect_pkg_manager() {
    if command -v apt-get &> /dev/null; then
        PKG_MANAGER="apt"
    elif command -v dnf &> /dev/null; then
        PKG_MANAGER="dnf"
    elif command -v yum &> /dev/null; then
        PKG_MANAGER="yum"
    elif command -v apk &> /dev/null; then
        PKG_MANAGER="apk"
    else
        echo "❌ Ошибка: Не поддерживаемый дистрибутив (нет apt, dnf, yum или apk)."
        exit 1
    fi
}

# Функция универсальной установки пакетов
install_packages() {
    detect_pkg_manager
    local packages=("$@")
    
    echo "📦 Используется менеджер пакетов: $PKG_MANAGER"
    
    case "$PKG_MANAGER" in
        apt)
            apt-get update -qq
            apt-get install -y "${packages[@]}"
            ;;
        dnf)
            dnf install -y "${packages[@]}"
            ;;
        yum)
            yum install -y "${packages[@]}"
            ;;
        apk)
            apk update
            apk add --no-cache "${packages[@]}"
            ;;
    esac
}

# 2. Проверка и установка зависимостей
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

MISSING_DEPS=0
DEPS_TO_INSTALL=()

# Базовые утилиты
if ! command -v curl &> /dev/null; then DEPS_TO_INSTALL+=("curl"); MISSING_DEPS=1; fi
if ! command -v zip &> /dev/null; then DEPS_TO_INSTALL+=("zip" "unzip"); MISSING_DEPS=1; fi

# Проверка Go
if ! command -v go &> /dev/null; then
    detect_pkg_manager
    if [ "$PKG_MANAGER" = "apk" ]; then
        DEPS_TO_INSTALL+=("go")
    else
        DEPS_TO_INSTALL+=("golang")
    fi
    MISSING_DEPS=1
fi

# Проверка Node.js выполняется отдельно через Nodesource для не-Alpine систем
if [ "$NODE_TOO_OLD" -eq 1 ]; then
    MISSING_DEPS=1
fi

# Кросс-компиляторы (Имена пакетов зависят от дистрибутива)
detect_pkg_manager
if [ "$NEED_CROSS_ARM64" -eq 1 ] && ! command -v aarch64-linux-gnu-gcc &> /dev/null; then
    case "$PKG_MANAGER" in
        apt) DEPS_TO_INSTALL+=("gcc-aarch64-linux-gnu") ;;
        dnf|yum) DEPS_TO_INSTALL+=("gcc-aarch64-linux-gnu") ;;
        apk) DEPS_TO_INSTALL+=("gcc-aarch64-none-elf") ;; # В Alpine может отличаться
    esac
    MISSING_DEPS=1
fi

if [ "$NEED_CROSS_WIN" -eq 1 ] && ! command -v x86_64-w64-mingw32-gcc &> /dev/null; then
    case "$PKG_MANAGER" in
        apt) DEPS_TO_INSTALL+=("mingw-w64") ;;
        dnf|yum) DEPS_TO_INSTALL+=("mingw64-gcc") ;;
        apk) DEPS_TO_INSTALL+=("mingw-w64-gcc") ;;
    esac
    MISSING_DEPS=1
fi

if [ "$MISSING_DEPS" -eq 1 ]; then
    if [ "$EUID" -ne 0 ]; then
        echo "❌ Зависимости отсутствуют. Запустите скрипт с правами суперпользователя: sudo bash $0 $TARGET"
        exit 1
    fi
    
    # Сначала ставим базовые пакеты, если они в списке
    if [ ${#DEPS_TO_INSTALL[@]} -gt 0 ]; then
        echo "🛠 Установка пакетов: ${DEPS_TO_INSTALL[*]}"
        install_packages "${DEPS_TO_INSTALL[@]}"
    fi
    
    # Установка Node.js 22 (если старый/нет)
    if [ "$NODE_TOO_OLD" -eq 1 ]; then
        echo "📦 Установка актуальной версии Node.js..."
        if [ "$PKG_MANAGER" = "apk" ]; then
            apk add --no-cache nodejs npm
        elif [ "$PKG_MANAGER" = "apt" ]; then
            apt-get remove -y nodejs npm libnode-dev 2>/dev/null || true
            curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && apt-get install -y nodejs
        else # dnf / yum
            dnf remove -y nodejs npm 2>/dev/null || true
            curl -fsSL https://rpm.nodesource.com/setup_22.x | bash - && dnf install -y nodejs
        fi
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
        # Проверяем имя компилятора в системе (для Alpine/других ОС)
        if command -v aarch64-linux-gnu-gcc &> /dev/null; then
            env_cc="CC=aarch64-linux-gnu-gcc"
        elif command -v aarch64-none-elf-gcc &> /dev/null; then
            env_cc="CC=aarch64-none-elf-gcc"
        fi
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
