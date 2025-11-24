FROM alpine:3.20

WORKDIR app
COPY xray-tools.sh .
# Нужные утилиты: curl + bash (если xray-tools.sh написан под bash)
RUN apk add --no-cache curl bash ca-certificates tzdata

# Копируем entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
RUN chmod +x xray-tools.sh


# cron внутри alpine использует crond (busybox)
CMD ["/entrypoint.sh"]