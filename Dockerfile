# Load the base image into the build host first. Supply prebuilt Linux
# binaries in release/docker. No dependencies are fetched during this build.
ARG OFFLINE_BASE=debian:bookworm-slim
FROM ${OFFLINE_BASE}
WORKDIR /app
COPY release/docker/ /app/
COPY DockerEntrypoint.sh /app/
RUN chmod 0755 /app/x-ui /app/DockerEntrypoint.sh /app/bin/xray-linux-*
ENV XUI_IN_DOCKER=true XUI_MAIN_FOLDER=/app XUI_ENABLE_FAIL2BAN=false
EXPOSE 2053
VOLUME ["/etc/x-ui"]
ENTRYPOINT ["/app/DockerEntrypoint.sh"]
