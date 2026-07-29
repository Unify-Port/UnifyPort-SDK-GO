#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
MANIFEST="${ROOT}/parity/manifest.gen.json"
ENGLISH="${ROOT}/docs/api-coverage.md"
CHINESE="${ROOT}/docs/zh-CN/api-coverage.md"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TEMP_DIR}"' EXIT

for required in "${MANIFEST}" "${ENGLISH}" "${CHINESE}"; do
  if [[ ! -f "${required}" ]]; then
    printf '[public:docs] missing required file: %s\n' "${required#"${ROOT}"/}" >&2
    exit 1
  fi
done

for pair in \
  "README.md:README.zh-CN.md" \
  "AGENTS.md:AGENTS.zh-CN.md" \
  "CONTRIBUTING.md:CONTRIBUTING.zh-CN.md" \
  "NOTICE.md:NOTICE.zh-CN.md" \
  "SECURITY.md:SECURITY.zh-CN.md" \
  "examples/README.md:examples/README.zh-CN.md"; do
  english_pair="${pair%%:*}"
  chinese_pair="${pair#*:}"
  if [[ ! -f "${ROOT}/${english_pair}" || ! -f "${ROOT}/${chinese_pair}" ]]; then
    printf '[public:docs] incomplete bilingual pair: %s\n' "${pair}" >&2
    exit 1
  fi
done

# 每个 canonical guide 都必须有同名中文版本，防止只更新一种语言。
for english_doc in "${ROOT}"/docs/*.md; do
  base="${english_doc##*/}"
  if [[ ! -f "${ROOT}/docs/zh-CN/${base}" ]]; then
    printf '[public:docs] missing Simplified Chinese guide: docs/zh-CN/%s\n' "${base}" >&2
    exit 1
  fi
done

# 本地 Markdown target 必须真实存在；外部 URL 和页内 anchor 由各自发布面验证。
while IFS= read -r -d '' markdown; do
  while IFS= read -r link; do
    target="${link#']('}"
    target="${target%%#*}"
    target="${target%% *}"
    case "${target}" in
      ""|http://*|https://*|mailto:*|\#*)
        continue
        ;;
    esac
    if [[ ! -e "$(dirname "${markdown}")/${target}" ]]; then
      printf '[public:docs] %s: missing local link target %s\n' \
        "${markdown#"${ROOT}"/}" "${target}" >&2
      exit 1
    fi
  done < <(LC_ALL=C grep -Eo '\]\([^)]+' "${markdown}" || true)
done < <(find "${ROOT}" -type f -name '*.md' -not -path "${ROOT}/.git/*" -print0)

# Manifest 是 coverage 的唯一枚举来源，文档只允许完整映射而不能维护另一份子集。
sed -n 's/^[[:space:]]*"operationId": "\([^"]*\)",/\1/p' "${MANIFEST}" | LC_ALL=C sort >"${TEMP_DIR}/manifest"
sed -n 's/^| `\([^`]*\)` |.*/\1/p' "${ENGLISH}" | LC_ALL=C sort >"${TEMP_DIR}/english"
sed -n 's/^| `\([^`]*\)` |.*/\1/p' "${CHINESE}" | LC_ALL=C sort >"${TEMP_DIR}/chinese"

operation_count="$(wc -l <"${TEMP_DIR}/manifest" | tr -d '[:space:]')"
if [[ "${operation_count}" != "64" ]]; then
  printf '[public:docs] manifest operation count is %s, want 64\n' "${operation_count}" >&2
  exit 1
fi

if ! diff -u "${TEMP_DIR}/manifest" "${TEMP_DIR}/english"; then
  printf '[public:docs] English API coverage differs from manifest\n' >&2
  exit 1
fi
if ! diff -u "${TEMP_DIR}/manifest" "${TEMP_DIR}/chinese"; then
  printf '[public:docs] Chinese API coverage differs from manifest\n' >&2
  exit 1
fi

printf '[public:docs] verified %s operations in both API coverage tables\n' "${operation_count}" >&2
