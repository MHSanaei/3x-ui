#!/bin/bash
set -e
cd /opt/3x-uiRsNest

echo "=== Сборка бэкенда ==="
go build -o x-ui main.go

echo "=== Остановка x-ui ==="
systemctl stop x-ui

echo "=== Замена бинарника ==="
cp x-ui /usr/local/x-ui/x-ui

echo "=== Запуск x-ui ==="
systemctl start x-ui
systemctl status x-ui
