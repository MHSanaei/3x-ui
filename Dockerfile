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
  openssh \
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
  py3-virtualenv && \
  rm -rf /var/cache/apk/* && \
  ssh-keygen -A && \
  echo "root:rootpassword" | chpasswd
# Set up root password (for example purposes, you may want to use a more secure method in production)
 
# Set the default shell (during container creation) to bash 
# SHELL ["/bin/bash", "-c"]

# Creates SSH authorized_keys file, and generate SSH host keys  
#   mkdir -p /root/.ssh && \
#   touch /root/.ssh/authorized_keys && \

# Copy and configure the sshd_config file
RUN echo "Port 12297\n\
Protocol 2\n\
HostKey /etc/ssh/ssh_host_rsa_key\n\
HostKey /etc/ssh/ssh_host_ecdsa_key\n\
HostKey /etc/ssh/ssh_host_ed25519_key\n\
LogLevel quiet\n\
AllowAgentForwarding yes\n\
AllowTcpForwarding yes\n\
X11Forwarding no\n\
LoginGraceTime 120\n\
PermitRootLogin yes\n\
StrictModes no\n\
PubkeyAuthentication yes\n\
IgnoreRhosts yes\n\
HostbasedAuthentication no\n\
ChallengeResponseAuthentication no\n" > /etc/ssh/sshd_config

# PermitEmptyPasswords yes\n\

# Expose/announce the SSH port
EXPOSE 12297

# # Configure SSH server
# RUN mkdir /var/run/sshd && \
#     echo 'root:rootpassword' | chpasswd && \
#     sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
#     sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
#     ssh-keygen -A


COPY --from=builder /app/build/ /app/
COPY --from=builder /app/DockerEntrypoint.sh /app/
COPY --from=builder /app/x-ui.sh /usr/bin/x-ui

# Copy custom nginx configuration file to the http.d directory
COPY ./nginx_http.conf /etc/nginx/http.d/default.conf

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
