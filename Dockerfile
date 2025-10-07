# ========================================================
# Stage: Builder
# ========================================================
# если 1.25 нет в DockerHub — ставь 1.22
FROM golang:1.22-alpine AS builder
WORKDIR /app
ARG TARGETARCH

RUN apk --no-cache --update add \
  build-base \
  gcc \
  wget \
  unzip

COPY . .

# если у тебя есть приватные модули — можно добавить go env+git config (не нужно, если всё публичное)
RUN go mod download

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

# соберём бинарь x-ui
RUN go build -ldflags "-w -s" -o build/x-ui main.go

# твой инициализатор, если он нужен
RUN ./DockerInit.sh "$TARGETARCH"

# ========================================================
# Stage: Final Image of 3x-ui
# ========================================================
FROM alpine:3.20
ENV TZ=Asia/Tehran
WORKDIR /app

RUN apk add --no-cache --update \
  ca-certificates \
  tzdata \
  fail2ban \
  bash \
  sqlite

# бинарь и скрипты
COPY --from=builder /app/build/ /app/
COPY --from=builder /app/DockerEntrypoint.sh /app/
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

# fail2ban (как у тебя)
RUN rm -f /etc/fail2ban/jail.d/alpine-ssh.conf \
  && cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local \
  && sed -i "s/^\[ssh\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
  && sed -i "s/^\[sshd\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
  && sed -i "s/#allowipv6 = auto/allowipv6 = auto/g" /etc/fail2ban/fail2ban.conf

RUN chmod +x \
  /app/DockerEntrypoint.sh \
  /app/x-ui \
  /usr/bin/x-ui

ENV XUI_ENABLE_FAIL2BAN="true"

# панель слушает 2053 (как в твоих настройках)
EXPOSE 2053

# смонтируем /etc/x-ui как data dir (как у тебя в compose)
VOLUME [ "/etc/x-ui" ]

# твой же entrypoint/cmd
ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]
CMD [ "./x-ui" ]
