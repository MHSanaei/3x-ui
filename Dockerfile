# ========================================================
# Stage: Builder
# ========================================================
FROM golang:1.25-alpine AS builder

ARG TARGETARCH
ARG XRAY_VERSION
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV XRAY_GEO_FILES_DIR="/app/xray-build"
ENV XRAY_BUILD_DIR="/app/build/x-ui"

WORKDIR /app

RUN apk add --no-cache --update \
  build-base \
  gcc \
  curl \
  unzip \
  bash

# Install xray-core and geodat files
RUN mkdir -p "$XRAY_GEO_FILES_DIR"
COPY lib/geo.sh "$XRAY_GEO_FILES_DIR"/
COPY lib/xray-tools.sh "$XRAY_GEO_FILES_DIR"/

RUN chmod +x "$XRAY_GEO_FILES_DIR"/xray-tools.sh \
    && chmod +x "$XRAY_GEO_FILES_DIR"/geo.sh
RUN "$XRAY_GEO_FILES_DIR"/xray-tools.sh install_xray_core "$TARGETARCH" "$XRAY_GEO_FILES_DIR"/bin "$XRAY_VERSION" \
    && "$XRAY_GEO_FILES_DIR"/geo.sh update_all_geofiles "$XRAY_GEO_FILES_DIR"/bin

# docker CACHE
COPY go.mod go.sum ./
RUN go mod download

# Faster build, no extra files or volumes copied
COPY config/ config/
COPY database/ database/
COPY logger/ logger/
COPY sub/ sub/
COPY web/ web/
COPY util/ util/
COPY xray/ xray/
COPY main.go ./

RUN go build -ldflags "-w -s" -o "$XRAY_BUILD_DIR" main.go

# ========================================================
# Stage: Final Image of 3x-ui
# ========================================================
FROM alpine

RUN apk add --no-cache \
  ca-certificates \
  tzdata \
  fail2ban \
  bash

WORKDIR /app

COPY DockerEntrypoint.sh ./
COPY --from=builder /app/build/x-ui ./
COPY --from=builder /app/xray-build/bin/ /tmp/xray/

# Configure fail2ban
RUN rm -f /etc/fail2ban/jail.d/alpine-ssh.conf \
  && cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local \
  && sed -i "s/^\[ssh\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
  && sed -i "s/^\[sshd\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
  && sed -i "s/#allowipv6 = auto/allowipv6 = auto/g" /etc/fail2ban/fail2ban.conf

RUN chmod +x /app/DockerEntrypoint.sh

EXPOSE 2053
VOLUME [ "/etc/x-ui" ]

ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]
