# Arch Linux

3x-ui имеет отдельный профиль установки и systemd-юнит для Arch Linux.

## Что учитывается

- Для pacman используется полное обновление -Syu, а не неподдерживаемый partial upgrade.
- Arch-специфичный systemd-юнит использует network-online.target.
- Повышен LimitNOFILE до 1048576 для большого числа одновременных соединений.
- TasksMax=infinity снимает искусственное ограничение systemd на количество задач процесса.
- OOMScoreAdjust=-100 снижает вероятность того, что панель будет выбрана первой при нехватке памяти.
- PrivateTmp=true изолирует временные файлы процесса.
- UMask=0027 ограничивает права новых файлов, создаваемых процессом.
- Автозапуск панели остаётся через systemd.

## Установка

На Arch Linux перед установкой рекомендуется выполнить обычное полное обновление системы:

```bash
sudo pacman -Syu
```

Затем используйте штатный установщик проекта. Для Arch он автоматически выбирает x-ui.service.arch.

После установки:

```bash
sudo systemctl status x-ui
sudo systemctl enable x-ui
sudo systemctl restart x-ui
```

Для просмотра журнала:

```bash
sudo journalctl -u x-ui -b
```

## Cron и сертификаты

Arch не запускает cron-демон автоматически после установки. Если используется acme.sh с cron-задачами, установщик ставит cronie; служба должна быть запущена:

```bash
sudo systemctl enable --now cronie
```

Если cron не нужен в вашей конфигурации, его можно не запускать.

## Проверка systemd

Полезные проверки после установки:

```bash
systemd-analyze verify /etc/systemd/system/x-ui.service
systemctl show x-ui --property=LimitNOFILE,TasksMax,OOMScoreAdjust,UMask,PrivateTmp
```

Полный CI проекта должен выполняться отдельно в чистой Arch-среде или CI runner. Из текущей среды проекта локальный CI не запускался.

## Режим малой памяти и одного CPU

Для небольших VPS Arch-юнит задаёт консервативные значения по умолчанию:

- `GOMAXPROCS=1` — Go runtime работает с одним CPU;
- `GOMEMLIMIT=256MiB` — runtime раньше запускает сборку мусора и старается удерживать память в заданном бюджете;
- `GOGC=80` — уменьшает рост heap относительно стандартного значения;
- `CPUQuota=100%` — systemd ограничивает сервис примерно одним CPU;
- `MemoryHigh=384M` — systemd подаёт memory pressure до достижения жёсткого лимита;
- `XUI_LOW_RESOURCE=true` — SQLite использует меньшие cache/mmap, файловое временное хранилище и пул до 2 соединений.

Это именно безопасный профиль по умолчанию, а не гарантия, что весь стек 3x-ui/Xray всегда уложится в 256 MiB. Xray и дочерние процессы могут потреблять дополнительную память. Если сервер имеет больше RAM, `GOMEMLIMIT` можно увеличить через `/etc/conf.d/x-ui`.
