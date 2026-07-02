#!/bin/bash

# Пути к боту и конфигу
BOT_DIR="/usr/local/x-ui/xray-bot"
ENV_FILE="$BOT_DIR/src/.env"
SERVICE_NAME="xray-bot"

# Цвета для красоты
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
PLAIN='\033[0m'

# Функция проверки статуса бота
check_status() {
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo -e "Статус бота: ${GREEN}ВКЛЮЧЕН (Работает)${PLAIN}"
    else
        echo -e "Статус бота: ${RED}ВЫКЛЮЧЕН (Остановлен)${PLAIN}"
    fi
}

# Функция изменения параметров .env (СТРОГИЙ РЕЖИМ)
manage_env() {
    if [ ! -f "$ENV_FILE" ]; then
        echo -e "${RED}Ошибка: Файл .env не найден по пути $ENV_FILE${PLAIN}"
        return
    fi
    
    echo -e "\n--- Текущие настройки .env ---"
    cat "$ENV_FILE"
    echo -e "-----------------------------\n"
    
    echo "Введите название переменной, которую хотите изменить (например, BOT_TOKEN):"
    read -r var_name
    
    if [ -z "$var_name" ]; then
        echo -e "${RED}Название не может быть пустым!${PLAIN}"
        return
    fi
    
    # СТРОГАЯ ПРОВЕРКА: Если поля изначально нет в файле — шлем нафиг, ничего не создаем
    if ! grep -q "^$var_name=" "$ENV_FILE"; then
        echo -e "${RED}Ошибка: Параметр '$var_name' отсутствует в .env файле! Создание новых полей запрещено.${PLAIN}"
        return
    fi
    
    echo "Введите новое значение для $var_name:"
    read -r var_value
    
    # Меняем значение существующей переменной
    sed -i "s|^$var_name=.*|$var_name=$var_value|" "$ENV_FILE"
    
    echo -e "${GREEN}Настройки успешно обновлены! Не забудьте перезапустить бота (пункт 3).${PLAIN}"
}

# Бесконечный цикл меню
while true; do
    clear
    echo "==================================="
    echo "    Управление Xray Ботом          "
    echo "==================================="
    check_status
    echo "==================================="
    echo "1. Включить бота (Start)"
    echo "2. Выключить бота (Stop)"
    echo "3. Перезапустить бота (Restart)"
    echo "4. Посмотреть логи бота"
    echo "5. Изменить параметры (.env)"
    echo "0. Выйти"
    echo "==================================="
    echo -n "Выберите пункт меню: "
    read -r choice

    case "$choice" in
        1)
            echo "Запуск службы бота..."
            sudo systemctl start "$SERVICE_NAME"
            sleep 1
            ;;
        2)
            echo "Остановка службы бота..."
            sudo systemctl stop "$SERVICE_NAME"
            sleep 1
            ;;
        3)
            echo "Перезапуск службы бота..."
            sudo systemctl restart "$SERVICE_NAME"
            sleep 1
            ;;
        4)
            echo "Отображение последних 50 строк логов (Нажмите Ctrl+C для выхода):"
            sudo journalctl -u "$SERVICE_NAME" -n 50 -f
            ;;
        5)
            manage_env
            echo -n "Нажмите Enter для продолжения..."
            read -r
            ;;
        0)
            echo "Выход из меню."
            exit 0
            ;;
        *)
            echo -e "${RED}Неверный пункт!${PLAIN}"
            sleep 1
            ;;
    esac
done
