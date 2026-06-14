#!/usr/bin/env bash
#
# build-snapshot.sh — build a reusable Amazon Lightsail snapshot of 3x-ui.
#
# Flow (mirrors the Packer golden-image model, via the Lightsail API):
#   1. create an Ubuntu Lightsail instance with snapshot-userdata.sh
#      (installs the panel, NO database, enables the first-boot unit)
#   2. wait for provisioning, then (optionally) pin a known panel port and run
#      the shared cleanup.sh (wipes any DB/creds/keys/host-keys/cloud-init state)
#   3. stop the instance and create an instance snapshot
#   4. delete the build instance (unless --keep-instance)
#
# Every instance you later launch from the snapshot generates its OWN unique
# credentials on first boot (see deploy/firstboot/). The snapshot is private to
# your AWS account.
#
# Requirements: awscli v2, jq, ssh. AWS credentials with Lightsail permissions.
# Usage:
#   deploy/lightsail/build-snapshot.sh --region eu-central-1 [options]
# Options:
#   --region <r>            AWS region (default: $AWS_REGION or eu-central-1)
#   --blueprint-id <id>     Lightsail blueprint (default: ubuntu_24_04)
#   --bundle-id <id>        Lightsail bundle/size (default: small_3_0)
#   --availability-zone <z> AZ (default: <region>a)
#   --panel-port <p>        Pin the panel port in the snapshot so you can pre-open
#                           it in the Lightsail firewall (default: random per instance)
#   --snapshot-name <n>     Snapshot name (default: 3x-ui-ubuntu-24.04-<timestamp>)
#   --keep-instance         Do not delete the build instance afterwards
set -euo pipefail

REGION="${AWS_REGION:-eu-central-1}"
BLUEPRINT="ubuntu_24_04"
BUNDLE="small_3_0"
AZ=""
PANEL_PORT=""
SNAPSHOT_NAME=""
KEEP_INSTANCE=0

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
STAMP="$(date +%Y%m%d-%H%M%S)"
INSTANCE_NAME="3xui-build-${STAMP}"
KEY_FILE=""

log() { echo "[build-snapshot] $*"; }
die() {
    echo "[build-snapshot] ERROR: $*" >&2
    exit 1
}

while [ $# -gt 0 ]; do
    case "$1" in
        --region) REGION="$2"; shift 2 ;;
        --blueprint-id) BLUEPRINT="$2"; shift 2 ;;
        --bundle-id) BUNDLE="$2"; shift 2 ;;
        --availability-zone) AZ="$2"; shift 2 ;;
        --panel-port) PANEL_PORT="$2"; shift 2 ;;
        --snapshot-name) SNAPSHOT_NAME="$2"; shift 2 ;;
        --keep-instance) KEEP_INSTANCE=1; shift ;;
        -h | --help) sed -n '2,40p' "$0"; exit 0 ;;
        *) die "unknown option: $1" ;;
    esac
done

[ -n "$AZ" ] || AZ="${REGION}a"
[ -n "$SNAPSHOT_NAME" ] || SNAPSHOT_NAME="3x-ui-ubuntu-24.04-${STAMP}"

for cmd in aws jq ssh; do
    command -v "$cmd" > /dev/null 2>&1 || die "'$cmd' is required"
done

SSH_OPTS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10 -o LogLevel=ERROR)

cleanup() {
    [ -n "$KEY_FILE" ] && rm -f "$KEY_FILE"
    if [ "$KEEP_INSTANCE" -eq 0 ]; then
        aws lightsail delete-instance --instance-name "$INSTANCE_NAME" --region "$REGION" > /dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

wait_state() {
    local want="$1" tries="${2:-60}" st
    for _ in $(seq 1 "$tries"); do
        st=$(aws lightsail get-instance-state --instance-name "$INSTANCE_NAME" --region "$REGION" \
            --query 'state.name' --output text 2> /dev/null || echo "")
        [ "$st" = "$want" ] && return 0
        sleep 5
    done
    return 1
}

log "creating build instance ${INSTANCE_NAME} (${BLUEPRINT}/${BUNDLE}) in ${REGION}..."
aws lightsail create-instances \
    --instance-names "$INSTANCE_NAME" \
    --availability-zone "$AZ" \
    --blueprint-id "$BLUEPRINT" \
    --bundle-id "$BUNDLE" \
    --user-data "file://${SCRIPT_DIR}/snapshot-userdata.sh" \
    --region "$REGION" > /dev/null

log "waiting for instance to run..."
wait_state running 60 || die "instance did not reach 'running'"

IP=$(aws lightsail get-instance --instance-name "$INSTANCE_NAME" --region "$REGION" \
    --query 'instance.publicIpAddress' --output text)
if [ -z "$IP" ] || [ "$IP" = "None" ]; then die "no public IP"; fi
log "instance IP: ${IP}"

KEY_FILE="$(mktemp)"
# download-default-key-pair returns the key in 'privateKeyBase64'. Despite the
# name, the CLI historically emits the plaintext PEM (-----BEGIN...); the API
# docs describe it as base64. Handle both: write PEM as-is, else base64-decode.
KEY_RAW="$(aws lightsail download-default-key-pair --region "$REGION" \
    --query 'privateKeyBase64' --output text)"
[ -n "$KEY_RAW" ] && [ "$KEY_RAW" != "None" ] || die "failed to download default key pair"
case "$KEY_RAW" in
    *-----BEGIN*) printf '%s\n' "$KEY_RAW" > "$KEY_FILE" ;;
    *) printf '%s' "$KEY_RAW" | base64 -d > "$KEY_FILE" 2> /dev/null \
        || die "private key is neither PEM nor valid base64" ;;
esac
grep -q -- "-----BEGIN" "$KEY_FILE" || die "downloaded key is not a valid PEM private key"
chmod 600 "$KEY_FILE"

log "waiting for provisioning to finish (this installs the panel)..."
ok=0
for _ in $(seq 1 72); do # ~12 min
    if ssh "${SSH_OPTS[@]}" -i "$KEY_FILE" "ubuntu@${IP}" \
        'test -f /var/lib/3xui-provision-done' 2> /dev/null; then
        ok=1
        break
    fi
    sleep 10
done
[ "$ok" -eq 1 ] || die "provisioning did not complete in time"
log "provisioning complete."

if [ -n "$PANEL_PORT" ]; then
    log "pinning panel port ${PANEL_PORT} (username/password stay random)..."
    ssh "${SSH_OPTS[@]}" -i "$KEY_FILE" "ubuntu@${IP}" \
        "echo 'XUI_PANEL_PORT=${PANEL_PORT}' | sudo tee -a /etc/default/x-ui >/dev/null"
fi

log "stripping instance state (shared cleanup.sh)..."
ssh "${SSH_OPTS[@]}" -i "$KEY_FILE" "ubuntu@${IP}" \
    'curl -fsSL https://raw.githubusercontent.com/MHSanaei/3x-ui/main/deploy/packer/scripts/cleanup.sh | sudo bash'

log "stopping instance..."
aws lightsail stop-instance --instance-name "$INSTANCE_NAME" --region "$REGION" > /dev/null
wait_state stopped 60 || die "instance did not stop"

log "creating snapshot ${SNAPSHOT_NAME}..."
aws lightsail create-instance-snapshot \
    --instance-name "$INSTANCE_NAME" \
    --instance-snapshot-name "$SNAPSHOT_NAME" \
    --region "$REGION" > /dev/null

log "waiting for snapshot to become available..."
snap_ok=0
for _ in $(seq 1 120); do # ~20 min
    state=$(aws lightsail get-instance-snapshot --instance-snapshot-name "$SNAPSHOT_NAME" \
        --region "$REGION" --query 'instanceSnapshot.state' --output text 2> /dev/null || echo "")
    [ "$state" = "available" ] && {
        snap_ok=1
        break
    }
    sleep 10
done
[ "$snap_ok" -eq 1 ] || die "snapshot did not become available"

log "DONE."
echo
echo "================================================================"
echo " Lightsail snapshot ready: ${SNAPSHOT_NAME}  (region ${REGION})"
echo "================================================================"
echo " Launch an instance from it:"
echo "   aws lightsail create-instances-from-snapshot \\"
echo "     --instance-snapshot-name ${SNAPSHOT_NAME} \\"
echo "     --instance-names my-3xui-1 --bundle-id ${BUNDLE} \\"
echo "     --availability-zone ${AZ} --region ${REGION}"
if [ -n "$PANEL_PORT" ]; then
    echo
    echo " Then open the panel port (pinned to ${PANEL_PORT}):"
    echo "   aws lightsail open-instance-public-ports --region ${REGION} \\"
    echo "     --instance-name my-3xui-1 \\"
    echo "     --port-info fromPort=${PANEL_PORT},toPort=${PANEL_PORT},protocol=TCP"
else
    echo
    echo " Each instance picks a RANDOM panel port. After it boots, read it from"
    echo "   sudo cat /etc/x-ui/credentials.txt"
    echo " and open that TCP port in the instance's Lightsail IPv4 firewall."
fi
echo "================================================================"
