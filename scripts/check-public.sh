#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ROOT="${1:-${DEFAULT_ROOT}}"

if [[ ! -d "${ROOT}" ]]; then
  printf '[public:check] root is not a directory\n' >&2
  exit 2
fi

declare -a LABELS=(
  "private key material"
  "well-known access token"
  "JSON Web Token"
  "production-like Device API key"
  "developer absolute path"
  "private network URL"
  "private repository metadata"
)

declare -a PATTERNS=(
  '-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----'
  '(gh[pousr]_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|npm_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_-]{20,}|xox(a|b|p|r|s)-[A-Za-z0-9-]{20,})'
  'eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}'
  'dk_(live|prod)_?[A-Za-z0-9_-]{16,}'
  '(/Users/[^/[:space:]]+/|/home/[^/[:space:]]+/|[A-Za-z]:\\Users\\[^\\[:space:]]+\\)'
  'https?://(10\.[0-9.]+|192\.168\.[0-9.]+|172\.(1[6-9]|2[0-9]|3[01])\.[0-9.]+|[A-Za-z0-9.-]+\.(corp|internal|lan|local))([:/]|$)'
  '(internalRepository|privateRepository|internalPath|localPath|workspacePath)[[:space:]]*[:=]'
)

scanned=0
violations=0

while IFS= read -r -d '' path; do
  relative="${path#"${ROOT}"/}"

  # 扫描规则文本必然命中自身；脚本由 review 和独立 fixture 测试覆盖。
  if [[ "${path}" == "${SCRIPT_DIR}/check-public.sh" ]]; then
    continue
  fi

  base="${relative##*/}"
  case "${base}" in
    .env|.env.*|id_rsa|id_ed25519|credentials.json|service-account.json|*.pem|*.p12|*.pfx)
      if [[ "${base}" != ".env.example" ]]; then
        printf '[public:check] %s: forbidden sensitive filename\n' "${relative}" >&2
        violations=$((violations + 1))
      fi
      ;;
  esac

  # NUL/非文本文件不做内容匹配，避免把编译产物误解码成公开文本。
  if ! LC_ALL=C grep -Iq . "${path}"; then
    continue
  fi

  scanned=$((scanned + 1))
  for index in "${!PATTERNS[@]}"; do
    if LC_ALL=C grep -Eiq -- "${PATTERNS[${index}]}" "${path}"; then
      printf '[public:check] %s: %s\n' "${relative}" "${LABELS[${index}]}" >&2
      violations=$((violations + 1))
    fi
  done

  if LC_ALL=C grep -Ei 'UNIFYPORT_DEVICE_API_KEY[[:space:]]*=' "${path}" \
    | LC_ALL=C grep -Eiv '(<[^>]+>|replace-|example|changeme|\$\{|os\.Getenv|LookupEnv|requiredEnv)' \
    >/dev/null; then
    printf '[public:check] %s: hard-coded Device API key assignment\n' "${relative}" >&2
    violations=$((violations + 1))
  fi
done < <(
  find "${ROOT}" \
    \( -type d \( -name .git -o -name coverage -o -name dist -o -name vendor -o -name node_modules -o -name .cache \) -prune \) \
    -o -type f -print0
)

if ((violations > 0)); then
  printf '[public:check] failed with %d finding(s)\n' "${violations}" >&2
  exit 1
fi

printf '[public:check] checked %d public text file(s)\n' "${scanned}" >&2
