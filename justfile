set shell := ["bash", "-cu"]

port := "2099"
user := "admin"
pass := "admin"
db_dir := "tmp/db"
log_dir := "tmp/logs"
bin_dir := "tmp/bin"
app_bin := "tmp/bin/3x-ui-dev"
cookie := "tmp/cookies/dev.cookie"

# Show available commands
help:
    just --list

# Create local temp folders used by dev commands
ensure-tmp:
    mkdir -p {{db_dir}} {{log_dir}} {{bin_dir}} tmp/cookies

# Initialize local DB and default dev credentials/port (safe to re-run)
init-dev: ensure-tmp
    XUI_DB_FOLDER="$PWD/{{db_dir}}" XUI_LOG_FOLDER="$PWD/{{log_dir}}" XUI_DEBUG=true \
      go run . setting -port {{port}} -username {{user}} -password {{pass}}

# Build local dev binary
build: ensure-tmp
    GOPROXY=direct go build -o {{app_bin}} .

# Run app in dev mode (tmp DB/logs)
run: ensure-tmp
    XUI_DB_FOLDER="$PWD/{{db_dir}}" XUI_LOG_FOLDER="$PWD/{{log_dir}}" XUI_DEBUG=true \
      go run . run

# Run with live reload using Air (reads .air.toml)
air: ensure-tmp
    air -c .air.toml

# Quick compile check for all packages
check:
    GOPROXY=direct go build ./...

# Login to local dev panel and save cookie for API testing
api-login: ensure-tmp
    curl -s -c {{cookie}} -d 'username={{user}}&password={{pass}}' "http://127.0.0.1:{{port}}/login"

# Example: fetch client-center inbounds via API
api-clients-inbounds: api-login
    curl -s -b {{cookie}} "http://127.0.0.1:{{port}}/panel/api/clients/inbounds"

# Example: fetch client-center master clients via API
api-clients-list: api-login
    curl -s -b {{cookie}} "http://127.0.0.1:{{port}}/panel/api/clients/list"

# Remove local temp artifacts
clean-tmp:
    rm -rf tmp


# Run all unit tests
test:
    GOPROXY=direct go test ./...

# Run static checks
vet:
    GOPROXY=direct go vet ./...

staticcheck:
    if command -v staticcheck >/dev/null 2>&1; then staticcheck ./...; else echo "staticcheck not installed"; fi

# Print cyclomatic complexity snapshot
complexity:
    if command -v gocyclo >/dev/null 2>&1; then gocyclo -avg $(rg --files -g '*.go'); else echo "gocyclo not installed"; fi
