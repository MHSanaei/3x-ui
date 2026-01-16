FROM alpine:3.20

ARG TARGETARCH
ARG XRAY_VERSION
ARG XRAY_BUILD_DIR

WORKDIR /app

RUN apk add --no-cache  \
    wget  \
    unzip  \
    curl \
    bash \
    ca-certificates  \
    tzdata


COPY xray-tools.sh entrypoint.sh cron-job-script.sh ./

RUN chmod +x /app/xray-tools.sh /app/entrypoint.sh /app/cron-job-script.sh \
    && mkdir -p "$XRAY_BUILD_DIR" \
    && ./xray-tools.sh install_xray_core "$TARGETARCH" "$XRAY_BUILD_DIR" "$XRAY_VERSION" \
    && ./xray-tools.sh update_geodata_in_docker "$XRAY_BUILD_DIR"

ENV XRAY_BUILD_DIR=${XRAY_BUILD_DIR}

#CMD ["/app/entrypoint.sh"] \
ENTRYPOINT ["/app/entrypoint.sh"]