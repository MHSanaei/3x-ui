# ========================================================
# Stage: Builder
# ========================================================
FROM --platform=linux/amd64 golang:1.23.2-bullseye AS builder
WORKDIR /app

# Устанавливаем необходимые пакеты
RUN apt-get update && apt-get install -y \
    build-essential \
    gcc \
    wget \
    unzip \
    tar \
    && rm -rf /var/lib/apt/lists/*

COPY . .

# Устанавливаем переменные окружения для компиляции
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV GOOS=linux
ENV GOARCH=amd64

# Сборка бинарного файла
RUN go build -o x-ui main.go
RUN ./DockerInit.sh "amd64"

# Копирование файлов из build_files
COPY build_files /app/build_files

# Создание директории build и каталога x-ui внутри неё
RUN mkdir -p build/x-ui

# Копирование файлов в каталог x-ui внутри build
RUN cp x-ui build/x-ui/
RUN cp -r build_files/* build/x-ui/

# Создание архива tar.gz, включая каталог x-ui
RUN tar -czvf x-ui-linux-amd64.tar.gz -C build x-ui

# ========================================================
# Stage: Final Image of x-ui
# ========================================================
FROM debian:bullseye-slim AS final
ENV TZ=Asia/Tehran
WORKDIR /app

RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    fail2ban \
    bash \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/build/x-ui/ /app/
COPY --from=builder /app/DockerEntrypoint.sh /app/
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

# Настройка fail2ban
RUN rm -f /etc/fail2ban/jail.d/defaults-debian.conf \
    && cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local \
    && sed -i "s/^\[ssh\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
    && sed -i "s/^\[sshd\]$/&\nenabled = false/" /etc/fail2ban/jail.local

RUN chmod +x \
    /app/DockerEntrypoint.sh \
    /app/x-ui \
    /usr/bin/x-ui

VOLUME [ "/etc/x-ui" ]
CMD [ "./x-ui" ]
ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]

# ========================================================
# Stage: Export Archive
# ========================================================
FROM scratch AS export-stage
COPY --from=builder /app/x-ui-linux-amd64.tar.gz /x-ui-linux-amd64.tar.gz
