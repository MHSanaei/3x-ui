# ========================================================
# Stage: Builder
# ========================================================
FROM golang:1.22-alpine AS builder
WORKDIR /app
ARG TARGETARCH

RUN apk --no-cache --update add \
  build-base \
  gcc \
  wget \
  unzip

COPY . .

ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
RUN go build -o build/x-ui main.go
RUN ./DockerInit.sh "$TARGETARCH"

# ========================================================
# Stage: Final Image of 3x-ui
# ========================================================
FROM alpine
ENV TZ=Asia/Tehran
WORKDIR /app

RUN apk add --no-cache --update \
  ca-certificates \
  tzdata \
  fail2ban \
  bash \
  bash-completion \
  bc \
  supercronic \
  curl \
  gawk \
  git \
  htop \
  iptables \
  iperf3 \
  iproute2 \
  jq \
  nano \
  netcat-openbsd \
  nginx \
  socat \
  sqlite \
  tcptraceroute \
  tcpdump \
  tmux \
  unzip \
  wget \
  python3 \
  py3-pip \
  py3-psutil \
  py3-curl \
  py3-pysocks \
  py3-dotenv \
  py3-cloudflare \
  py3-virtualenv
 # openssh \
  # nginx-mod-stream \
  
SHELL ["/bin/bash", "-c"]

# Copy custom nginx configuration file to the http.d directory
COPY ./nginx_http.conf /etc/nginx/http.d/

## Set up the SSH keys from an environment variable
#ENV AUTHORIZED_KEYS=""
#RUN echo "${AUTHORIZED_KEYS}" > /root/.ssh/authorized_keys && \
#    chmod 600 /root/.ssh/authorized_keys

## Configure SSH daemon
#RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
#    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
    

# # Configure SSH server
# RUN mkdir /var/run/sshd && \
#     echo 'root:rootpassword' | chpasswd && \
#     sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
#     sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
#     ssh-keygen -A


COPY --from=builder /app/build/ /app/
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

VOLUME [ "/etc/x-ui" ]
CMD [ "./x-ui" ]
ENTRYPOINT [ "/app/DockerEntrypoint.sh" ]
