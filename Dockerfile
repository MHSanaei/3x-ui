Hi, I'm trying to fix my Fly.io deployment. There's a build error on my Dockerfile.

Please fix my Dockerfile for me to the best of your ability. And give me the updated version so I can paste in Fly.io web deployment UI.

Error logs:

```
flyctl deploy --build-only --push -a 3x-ui--cxykg --image-label deployment-d293a6140906d8d4a65e23e6864d86ef --config fly.toml
==> Verifying app config
Validating fly.toml
[32m✓[0m Configuration is valid
--> Verified app config
==> Building image
Waiting for depot builder...

==> Building image with Depot
--> build:  (​)
#1 [internal] load build definition from Dockerfile
#1 transferring dockerfile: 2.13kB done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/golang:1.22-alpine
#2 DONE 0.2s

#3 [internal] load metadata for docker.io/library/alpine:latest
#3 DONE 0.2s

#4 [internal] load .dockerignore
#4 transferring context: 2B 0.0s done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052
#5 resolve docker.io/library/golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052 done
#5 DONE 0.0s

#6 [stage-1 1/9] FROM docker.io/library/alpine:latest@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
#6 resolve docker.io/library/alpine:latest@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 done
#6 DONE 0.0s

#7 [stage-1 3/9] RUN apk add --no-cache --update   ca-certificates   tzdata   fail2ban   bash   curl   openssl
#7 CACHED

#8 [stage-1 2/9] WORKDIR /app
#8 CACHED

#9 [stage-1 4/9] RUN mkdir -p /app/bin     && curl -L -o /app/bin/geoip.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat     && curl -L -o /app/bin/geosite.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
#9 CACHED

#10 [internal] load build context
#10 ...

#11 [builder 2/6] WORKDIR /app
#11 CACHED

#12 [builder 3/6] RUN apk --no-cache --update add   build-base   gcc   curl   unzip
#12 CACHED

#10 [internal] load build context
#10 transferring context: 58.79MB 0.8s done
#10 ...

#13 [builder 4/6] COPY . .
#13 DONE 0.3s

#14 [builder 5/6] RUN go build -ldflags "-w -s" -o build/x-ui main.go
#14 0.080 go: go.mod requires go >= 1.25.7 (running go 1.22.12; GOTOOLCHAIN=local)
#14 ERROR: process "/bin/sh -c go build -ldflags \"-w -s\" -o build/x-ui main.go" did not complete successfully: exit code: 1

#10 [internal] load build context
------
 > [builder 5/6] RUN go build -ldflags "-w -s" -o build/x-ui main.go:
0.080 go: go.mod requires go >= 1.25.7 (running go 1.22.12; GOTOOLCHAIN=local)
------
==> Building image
Waiting for depot builder...

==> Building image with Depot
--> build:  (​)
#1 [internal] load build definition from Dockerfile
#1 transferring dockerfile: 2.13kB 0.0s done
#1 DONE 0.0s

#2 [internal] load metadata for docker.io/library/golang:1.22-alpine
#2 DONE 0.2s

#3 [internal] load metadata for docker.io/library/alpine:latest
#3 DONE 0.2s

#4 [internal] load .dockerignore
#4 transferring context: 2B done
#4 DONE 0.0s

#5 [builder 1/6] FROM docker.io/library/golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052
#5 resolve docker.io/library/golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052 done
#5 DONE 0.0s

#6 [stage-1 1/9] FROM docker.io/library/alpine:latest@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659
#6 resolve docker.io/library/alpine:latest@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 done
#6 DONE 0.0s

#7 [stage-1 3/9] RUN apk add --no-cache --update   ca-certificates   tzdata   fail2ban   bash   curl   openssl
#7 CACHED

#8 [stage-1 2/9] WORKDIR /app
#8 CACHED

#9 [stage-1 4/9] RUN mkdir -p /app/bin     && curl -L -o /app/bin/geoip.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat     && curl -L -o /app/bin/geosite.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
#9 CACHED

#10 [internal] load build context
#10 transferring context: 16.80kB 0.0s done
#10 DONE 0.0s

#11 [builder 3/6] RUN apk --no-cache --update add   build-base   gcc   curl   unzip
#11 CACHED

#12 [builder 2/6] WORKDIR /app
#12 CACHED

#13 [builder 4/6] COPY . .
#13 CACHED

#14 [builder 5/6] RUN go build -ldflags "-w -s" -o build/x-ui main.go
#14 0.079 go: go.mod requires go >= 1.25.7 (running go 1.22.12; GOTOOLCHAIN=local)
#14 ERROR: process "/bin/sh -c go build -ldflags \"-w -s\" -o build/x-ui main.go" did not complete successfully: exit code: 1
------
 > [builder 5/6] RUN go build -ldflags "-w -s" -o build/x-ui main.go:
0.079 go: go.mod requires go >= 1.25.7 (running go 1.22.12; GOTOOLCHAIN=local)
------
Error: failed to fetch an image or build from source: error building: failed to solve: process "/bin/sh -c go build -ldflags \"-w -s\" -o build/x-ui main.go" did not complete successfully: exit code: 1
Dockerfile failed to build error
unsuccessful command 'flyctl deploy --build-only --push -a 3x-ui--cxykg --image-label deployment-d293a6140906d8d4a65e23e6864d86ef --config fly.toml'
```

Dockerfile


```
# ========================================================
# Stage: Builder
# ========================================================
FROM golang:1.22-alpine AS builder
WORKDIR /app
ARG TARGETARCH

RUN apk --no-cache --update add \
  build-base \
  gcc \
  curl \
  unzip

COPY . .

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN go build -ldflags "-w -s" -o build/x-ui main.go
RUN if [ -f "./DockerInit.sh" ]; then chmod +x ./DockerInit.sh && ./DockerInit.sh "$TARGETARCH"; fi

# ========================================================
# Stage: Final Image of 3x-ui
# ========================================================
FROM alpine:latest
ENV TZ=Asia/Cairo
WORKDIR /app

RUN apk add --no-cache --update \
  ca-certificates \
  tzdata \
  fail2ban \
  bash \
  curl \
  openssl

# --------------------------------------------------------
# [هام] إضافة ملفات GeoIP و GeoSite لتجنب توقف السيرفر مستقبلاً
# --------------------------------------------------------
RUN mkdir -p /app/bin \
    && curl -L -o /app/bin/geoip.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat \
    && curl -L -o /app/bin/geosite.dat https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat

COPY --from=builder /app/build/ /app/
# تأكدنا هنا من نسخ السكربتات الضرورية
COPY --from=builder /app/DockerEntrypoint.sh /app/
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

# Configure fail2ban
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
ENV XRAY_LOCATION_ASSET=/app/bin/
EXPOSE 2053
VOLUME [ "/etc/x-ui" ]
CMD [ "./x-ui" ]
ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]

```


fly.toml


```
# fly.toml app configuration file generated for 3x-ui--cxykg on 2026-02-15T05:54:59Z
#
# See https://fly.io/docs/reference/configuration/ for information about how to use this file.
#

app = '3x-ui--cxykg'
primary_region = 'iad'

[build]

[http_service]
  internal_port = 2053
  force_https = true
  auto_stop_machines = 'stop'
  auto_start_machines = true
  min_machines_running = 0
  processes = ['app']

[[vm]]
  memory = '1gb'
  cpu_kind = 'shared'
  cpus = 1
  memory_mb = 1024

```

