#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE_ROOT="$(mktemp -d)"
trap 'rm -rf "${FIXTURE_ROOT}"' EXIT

expect_rejected() {
  local label="$1"
  local fixture="$2"
  if bash "${SCRIPT_DIR}/check-public.sh" "${fixture}" >/dev/null 2>&1; then
    printf '[public:check:test] expected rejection for %s\n' "${label}" >&2
    exit 1
  fi
}

# 每类规则使用独立 fixture，避免一个命中掩盖其他扫描规则的回归。
safe_root="${FIXTURE_ROOT}/safe"
mkdir -p "${safe_root}"
printf 'UNIFYPORT_DEVICE_API_KEY=<api-key>\n' >"${safe_root}/safe.env.example"
bash "${SCRIPT_DIR}/check-public.sh" "${safe_root}" >/dev/null 2>&1

# 高风险字面量分段构造，避免测试源码本身成为公开扫描误报。
private_key_root="${FIXTURE_ROOT}/private-key"
mkdir -p "${private_key_root}"
printf '%s%s\n' '-----BEGIN ' 'PRIVATE KEY-----' >"${private_key_root}/leak.txt"
expect_rejected "private key" "${private_key_root}"

token_root="${FIXTURE_ROOT}/token"
mkdir -p "${token_root}"
printf '%s%s\n' 'gh' 'p_12345678901234567890' >"${token_root}/leak.txt"
expect_rejected "access token" "${token_root}"

jwt_root="${FIXTURE_ROOT}/jwt"
mkdir -p "${jwt_root}"
printf '%s%s%s\n' 'eyJ12345678.' 'abcdefgh.' 'ABCDEFGH' >"${jwt_root}/leak.txt"
expect_rejected "JWT" "${jwt_root}"

api_key_root="${FIXTURE_ROOT}/production-key"
mkdir -p "${api_key_root}"
printf '%s%s%s\n' 'dk_' 'live_' '1234567890abcdef' >"${api_key_root}/leak.txt"
expect_rejected "production-like API key" "${api_key_root}"

assignment_root="${FIXTURE_ROOT}/key-assignment"
mkdir -p "${assignment_root}"
printf '%s%s\n' 'UNIFYPORT_DEVICE_API_' 'KEY=synthetic-but-hard-coded' >"${assignment_root}/leak.txt"
expect_rejected "hard-coded API key assignment" "${assignment_root}"

absolute_path_root="${FIXTURE_ROOT}/absolute-path"
mkdir -p "${absolute_path_root}"
printf '%s%s\n' '/Users/' 'synthetic/workspace' >"${absolute_path_root}/leak.txt"
expect_rejected "developer absolute path" "${absolute_path_root}"

private_url_root="${FIXTURE_ROOT}/private-url"
mkdir -p "${private_url_root}"
printf '%s%s\n' 'https://' '10.0.0.1/device' >"${private_url_root}/leak.txt"
expect_rejected "private network URL" "${private_url_root}"

metadata_root="${FIXTURE_ROOT}/private-metadata"
mkdir -p "${metadata_root}"
printf '%s%s\n' 'internal' 'Repository=value' >"${metadata_root}/leak.txt"
expect_rejected "private repository metadata" "${metadata_root}"

sensitive_name_root="${FIXTURE_ROOT}/sensitive-filename"
mkdir -p "${sensitive_name_root}"
printf 'synthetic\n' >"${sensitive_name_root}/credentials.json"
expect_rejected "sensitive filename" "${sensitive_name_root}"

printf '[public:check:test] fixture checks passed\n' >&2
