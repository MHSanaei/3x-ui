# ARG XRAY_VERSION=1.8.0

# Build stage
FROM golang:1.20.3-alpine3.17 AS build
WORKDIR /app
RUN apk update && apk add --no-cache --update gcc build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build main.go

# Runtime stage
FROM alpine:3.17.3
# ARG XRAY_VERSION
ENV TZ=Asia/Tehran
WORKDIR /app
# RUN useradd -D -g '' xui
RUN apk update && apk add --no-cache --update ca-certificates tzdata && update-ca-certificates

# Download xray-core binary and install it to /app/bin
# ADD https://github.com/XTLS/Xray-core/releases/download/v${XRAY_VERSION}/Xray-linux-64.zip /tmp/xray.zip
ADD https://github.com/mhsanaei/Xray-core/releases/latest/download/Xray-linux-64.zip /tmp/xray.zip
RUN unzip /tmp/xray.zip -d bin && rm /tmp/xray.zip && mv bin/xray bin/xray-linux-amd64

# Download latest rule files
ADD https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
    https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat \
    https://github.com/bootmortis/iran-hosted-domains/releases/latest/download/iran.dat \
    bin/
COPY --from=build  /app/main /app/x-ui
VOLUME [ "/etc/x-ui" ]
# USER xui
ENTRYPOINT ["/bin/sh", "-c", "ln -sf /proc/1/fd/1 /app/access.log; /app/x-ui"]
# CMD [ "/app/x-ui" ]
