# syntax=docker/dockerfile:latest
FROM --platform=$BUILDPLATFORM golang:latest AS build

# Build xray-core
WORKDIR /src
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -o xray -trimpath -buildvcs=false -gcflags="all=-l=4" -ldflags "-s -w -buildid=" ./main

# Download geodat into a staging directory
ADD https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/geoip.dat /tmp/geodat/geoip.dat
ADD https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/geosite.dat /tmp/geodat/geosite.dat

RUN mkdir -p /tmp/empty

# Create config files with empty JSON content
RUN mkdir -p /tmp/usr/local/etc/xray
RUN cat <<EOF >/tmp/usr/local/etc/xray/00_log.json
{
  "log": {
    "error": "/var/log/xray/error.log",
    "loglevel": "warning",
    "access": "none",
    "dnsLog": false
  }
}
EOF
RUN echo '{}' >/tmp/usr/local/etc/xray/01_api.json
RUN echo '{}' >/tmp/usr/local/etc/xray/02_dns.json
RUN echo '{}' >/tmp/usr/local/etc/xray/03_routing.json
RUN echo '{}' >/tmp/usr/local/etc/xray/04_policy.json
RUN echo '{}' >/tmp/usr/local/etc/xray/05_inbounds.json
RUN echo '{}' >/tmp/usr/local/etc/xray/06_outbounds.json
RUN echo '{}' >/tmp/usr/local/etc/xray/07_transport.json
RUN echo '{}' >/tmp/usr/local/etc/xray/08_stats.json
RUN echo '{}' >/tmp/usr/local/etc/xray/09_reverse.json

# Create log files
RUN mkdir -p /tmp/var/log/xray && touch \
  /tmp/var/log/xray/access.log \
  /tmp/var/log/xray/error.log

# Build finally image
FROM gcr.io/distroless/static:nonroot

COPY --from=build --chown=0:0 --chmod=755 /src/xray /usr/local/bin/xray
COPY --from=build --chown=0:0 --chmod=755 /tmp/empty /usr/local/share/xray
COPY --from=build --chown=0:0 --chmod=644 /tmp/geodat/*.dat /usr/local/share/xray/
COPY --from=build --chown=0:0 --chmod=755 /tmp/empty /usr/local/etc/xray
COPY --from=build --chown=0:0 --chmod=644 /tmp/usr/local/etc/xray/*.json /usr/local/etc/xray/
COPY --from=build --chown=0:0 --chmod=755 /tmp/empty /var/log/xray
COPY --from=build --chown=65532:65532 --chmod=600 /tmp/var/log/xray/*.log /var/log/xray/

VOLUME /usr/local/etc/xray
VOLUME /var/log/xray

ARG TZ=Etc/UTC
ENV TZ=$TZ

ENTRYPOINT [ "/usr/local/bin/xray" ]
CMD [ "-confdir", "/usr/local/etc/xray/" ]
