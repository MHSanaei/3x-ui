# ========================================================
# Stage: Builder
# ========================================================
FROM golang:1.22-alpine AS builder
WORKDIR /app
ARG TARGETARCH

# تحديث وتثبيت أدوات البناء
RUN apk --no-cache --update add \
    build-base \
    gcc \
    curl \
    unzip

COPY . .

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

# بناء التطبيق
RUN go build -ldflags "-w -s" -o build/x-ui main.go

# تشغيل سكربت DockerInit إذا كان موجوداً
RUN if [ -f "./DockerInit.sh" ]; then chmod +x ./DockerInit.sh && ./DockerInit.sh "$TARGETARCH"; fi

# ========================================================
# Stage: Final Image of 3x-ui
# ========================================================
FROM alpine:latest
ENV TZ=Asia/Cairo
WORKDIR /app

# تثبيت الحزم الأساسية
RUN apk add --no-cache --update \
    ca-certificates \
    tzdata \
    fail2ban \
    bash \
    curl \
    openssl

# --------------------------------------------------------
# الإصلاح: تحميل ملفات GeoIP و Geosite الضرورية
# --------------------------------------------------------
RUN mkdir -p /app/bin \
    && curl -L -o /app/bin/geoip.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
    && curl -L -o /app/bin/geosite.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat

# نسخ ملفات البناء من المرحلة السابقة
COPY --from=builder /app/build/ /app/
# (تأكد من وجود هذه الملفات في مشروعك أو سيتم تجاهلها إن لم تكن موجودة في الـ builder)
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

# إعداد fail2ban (اختياري، لتقليل حجم الصورة يمكن حذفه إذا لم تستخدمه)
RUN rm -f /etc/fail2ban/jail.d/alpine-ssh.conf \
    && cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local \
    && sed -i "s/^\[ssh\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
    && sed -i "s/^\[sshd\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
    && sed -i "s/#allowipv6 = auto/allowipv6 = auto/g" /etc/fail2ban/fail2ban.conf

# منح صلاحيات التنفيذ
RUN chmod +x \
    /app/x-ui \
    /usr/bin/x-ui

ENV XUI_ENABLE_FAIL2BAN="true"
# إخبار النظام بمكان ملفات الـ Assets (احتياطياً)
ENV XRAY_LOCATION_ASSET=/app/bin/

EXPOSE 2053
VOLUME [ "/etc/x-ui" ]

CMD [ "./x-ui" ]
