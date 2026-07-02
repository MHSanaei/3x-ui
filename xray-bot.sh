#!/bin/bash

# Пути к боту и конфигу (подставь свои, если они другие)
BOT_DIR="/usr/local/x-ui/xray-bot"
ENV_FILE="$BOT_DIR/src/.env"
SERVICE_NAME="xray-bot" # Предполагаем, что бот работает как служба systemd

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

# Функция изменения параметров .env
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
    
    # Проверяем, есть ли такая переменная
    if ! grep -q "^$var_name=" "$ENV_FILE"; then
        echo -e "${YELLOW}Переменная не найдена. Она будет создана.${PLAIN}"
    fi
    
    echo "Введите новое значение для $var_name:"
    read -r var_value
    
    # Если переменная есть — меняем, если нет — дописываем в конец
    if grep -q "^$var_name=" "$ENV_FILE"; then
        sed -i "s|^$var_name=.*|$var_name=$var_value|" "$ENV_FILE"
    else
        echo "$var_name=$var_value" >> "$ENV_FILE"
    fi
    
    echo -e "${GREEN}Настройки успешно обновлены! Не забудьте перезапустить бота.${PLAIN}"
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
