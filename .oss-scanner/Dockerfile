# Build environment for Anthropic's OSS Scanner (https://github.com/anthropics/oss-scanner).
# The scanner builds this with the repository as the context, then audits the image offline:
# everything the build, the tests and a reproducer need must be fetched here.

FROM node:26-trixie-slim AS node

FROM golang:1.27.3-trixie

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=1 \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE" \
    GOTOOLCHAIN=local

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential ca-certificates curl git jq procps sqlite3 unzip \
    && rm -rf /var/lib/apt/lists/*

# Node 26 is required: `make gen` imports .ts files directly and needs its type stripping.
COPY --from=node /usr/local/bin/node /usr/local/bin/node
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx \
    && node --version && npm --version

# scanner contract: the checkout lives inside the image, at /src
COPY . /src
WORKDIR /src

RUN go mod download && go mod verify

# The Go binary embeds the Vite build from internal/web/dist, so the frontend goes first.
# HUSKY=0 skips the git-hook install, which needs a .git directory the build context may lack.
RUN cd frontend && HUSKY=0 npm ci && npm run build

# Debug info and no inlining, so stack traces and debuggers map straight to the source.
RUN go build -gcflags=all="-N -l" -o /usr/local/bin/x-ui . \
    && ln -s /usr/local/bin/x-ui /src/x-ui

# Xray-core, the mtg-multi MTProto sidecar and the geo databases, exactly as the release image ships them.
RUN ./DockerInit.sh "$(dpkg --print-architecture)" \
    && ln -s "/src/build/bin/xray-linux-$(dpkg --print-architecture)" /usr/local/bin/xray \
    && xray version

# Turns on the xray gRPC end-to-end tests (internal/xray, internal/amneziawgnet) against the real core.
ENV XRAY_E2E_BINARY=/usr/local/bin/xray

# Baseline tests: also warms the build cache. A failure is logged, never fatal.
RUN go test -count=1 -timeout 30m ./... > /src/.oss-scanner-go-test.log 2>&1 \
    || echo "WARNING: go test reported failures, see /src/.oss-scanner-go-test.log"
RUN cd frontend && npx vitest run --project unit --project components > /src/.oss-scanner-vitest.log 2>&1 \
    || echo "WARNING: vitest reported failures, see /src/.oss-scanner-vitest.log"

# Reproducers and patches must build with the network gone.
ENV GOPROXY=off \
    GOSUMDB=off \
    GOFLAGS=-mod=readonly \
    npm_config_offline=true

CMD ["bash"]
