# ========================================================
# Stage: Builder
# ========================================================
FROM --platform=$BUILDPLATFORM golang:1.21.5-bookworm AS builder
WORKDIR /app

ARG TARGETARCH
ARG TARGETOS

RUN export DEBIAN_FRONTEND=noninteractive \
 && apt-get update -qq \
 && apk add --update --no-cache -qqy \
        build-base \
        gcc \
        wget \
        unzip \
 && if [ "${TARGETARCH}" = 'arm64' ]; then \
        apt-get install -qqy gcc-aarch64-linux-gnu; \
    fi \
 && apt-get clean \
 && rm -rf /var/cache/apt

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
# Copy everything else
COPY . .

ENV CGO_ENABLED=1
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
# Build with arm64 crosscompilation if required
RUN if [ "${TARGETARCH}" = 'arm64' ]; then \
        export CC=aarch64-linux-gnu-gcc; \
    fi \
 && go build -a \
        -ldflags="-s -w -extldflags=-static" \
        -trimpath -o build/x-ui main.go

RUN ./DockerInit.sh "$TARGETARCH"

# ========================================================
# Stage: Final Image of 3x-ui
# ========================================================
FROM --platform=$TARGETPLATFORM alpine
ENV TZ=Asia/Tehran
WORKDIR /app

RUN apk add --no-cache --update \
        ca-certificates \
        tzdata \
        fail2ban

COPY --from=builder /app/build/ /app/
COPY --from=builder /app/DockerEntrypoint.sh /app/
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

# Configure fail2ban
RUN rm -f /etc/fail2ban/jail.d/alpine-ssh.conf \
 && cp /etc/fail2ban/jail.conf /etc/fail2ban/jail.local \
 && sed -i "s/^\[ssh\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
 && sed -i "s/^\[sshd\]$/&\nenabled = false/" /etc/fail2ban/jail.local \
 && sed -i "s/#allowipv6 = auto/allowipv6 = auto/g" /etc/fail2ban/fail2ban.conf

RUN chmod 0755 \
    /app/DockerEntrypoint.sh \
    /app/x-ui \
    /usr/bin/x-ui

VOLUME [ "/etc/x-ui" ]
ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]
