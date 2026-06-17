#!/bin/sh

# Start fail2ban with the dune-ipl jail
if [ "$DUNE_ENABLE_FAIL2BAN" = "true" ]; then
    LOG_FOLDER="${DUNE_LOG_FOLDER:-/var/log/dune}"
    mkdir -p "$LOG_FOLDER"
    touch "$LOG_FOLDER/duneipl.log" "$LOG_FOLDER/duneipl-banned.log"

    mkdir -p /etc/fail2ban/jail.d /etc/fail2ban/filter.d /etc/fail2ban/action.d

    cat > /etc/fail2ban/jail.d/dune-ipl.conf << EOF
[dune-ipl]
enabled=true
backend=auto
filter=dune-ipl
action=dune-ipl
logpath=$LOG_FOLDER/duneipl.log
maxretry=1
findtime=32
bantime=30m
EOF

    cat > /etc/fail2ban/filter.d/dune-ipl.conf << 'EOF'
[Definition]
datepattern = ^%%Y/%%m/%%d %%H:%%M:%%S
failregex   = \[LIMIT_IP\]\s*Email\s*=\s*<F-USER>.+</F-USER>\s*\|\|\s*Disconnecting OLD IP\s*=\s*<ADDR>\s*\|\|\s*Timestamp\s*=\s*\d+
ignoreregex =
EOF

    # Ports to exempt from the ban so an over-limit proxy client can never lock
    # the administrator out of SSH or the panel. The ban still covers every other
    # TCP port (including all Xray inbounds), so IP-limit keeps working for inbounds
    # added later without regenerating these files.
    SSH_PORTS=$(grep -oE '^[[:space:]]*Port[[:space:]]+[0-9]+' /etc/ssh/sshd_config 2>/dev/null | grep -oE '[0-9]+' | paste -sd, -)
    [ -z "$SSH_PORTS" ] && SSH_PORTS="22"
    PANEL_PORT=$(/app/dune setting -show true 2>/dev/null | grep -Eo 'port: .+' | awk '{print $2}')
    EXEMPT_PORTS="$SSH_PORTS"
    [ -n "$PANEL_PORT" ] && EXEMPT_PORTS="$EXEMPT_PORTS,$PANEL_PORT"

    cat > /etc/fail2ban/action.d/dune-ipl.conf << EOF
[INCLUDES]
before = iptables-allports.conf

[Definition]
actionstart = <iptables> -N f2b-<name>
              <iptables> -A f2b-<name> -j <returntype>
              <iptables> -I <chain> -j f2b-<name>

actionstop = <iptables> -D <chain> -j f2b-<name>
             <actionflush>
             <iptables> -X f2b-<name>

actioncheck = <iptables> -n -L <chain> | grep -q 'f2b-<name>[ \t]'

actionban = <iptables> -I f2b-<name> 1 -s <ip> -p tcp -m multiport ! --dports <exemptports> -j <blocktype>
            <iptables> -I f2b-<name> 1 -s <ip> -p udp -m multiport ! --dports <exemptports> -j <blocktype>
            echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   BAN   [Email] = <F-USER> [IP] = <ip> banned for <bantime> seconds." >> $LOG_FOLDER/duneipl-banned.log

actionunban = <iptables> -D f2b-<name> -s <ip> -p tcp -m multiport ! --dports <exemptports> -j <blocktype>
              <iptables> -D f2b-<name> -s <ip> -p udp -m multiport ! --dports <exemptports> -j <blocktype>
              echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   UNBAN   [Email] = <F-USER> [IP] = <ip> unbanned." >> $LOG_FOLDER/duneipl-banned.log

[Init]
name = default
chain = INPUT
exemptports = $EXEMPT_PORTS
EOF

    fail2ban-client -x start
fi

# Run dune
exec /app/dune
