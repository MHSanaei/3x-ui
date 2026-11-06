FROM --platform=$BUILDPLATFORM golang:1.27 AS artifacts
ARG TARGETARCH
WORKDIR /src
COPY . .
RUN go run ./tools/wattunnel-release -target linux-${TARGETARCH} -out /out

FROM ubuntu:24.04
ARG TARGETARCH
RUN apt-get update && apt-get install -y --no-install-recommends iproute2 nftables python3 curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=artifacts /out /opt/wattunnel
COPY scripts/wattunnel-lab.sh scripts/wattunnel-lab-probe.py /lab/
ENV WATTUN_E2E_BINARY=/opt/wattunnel/wattun-linux-${TARGETARCH}
ENV WATTUN_LAB_OUTPUT=/results
ENTRYPOINT ["/lab/wattunnel-lab.sh"]
