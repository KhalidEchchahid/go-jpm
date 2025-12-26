#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
JPM_BIN="${JPM_BIN:-${ROOT_DIR}/jpm}"
# Single-project mode (legacy) still supported via PROJECT_DIR,
# but by default we benchmark the three fixtures under bench/projects.
PROJECT_DIR="${PROJECT_DIR:-}"
RUNS="${RUNS:-10}"
RESULTS_DIR="${RESULTS_DIR:-${ROOT_DIR}/bench/results}"

LATEST_DIR="${RESULTS_DIR}/latest"
TS_DIR="${RESULTS_DIR}/$(date -u +%Y%m%dT%H%M%SZ)"

mkdir -p "${LATEST_DIR}" "${TS_DIR}"

if [[ ! -x "${JPM_BIN}" ]]; then
  echo "[bench] JPM binary not found at ${JPM_BIN}. Building..." >&2
  (cd "${ROOT_DIR}" && go build -o "${JPM_BIN}" ./cmd/jpm)
fi

uname_s="$(uname -s 2>/dev/null || true)"
uname_m="$(uname -m 2>/dev/null || true)"
go_ver="$(go version 2>/dev/null || true)"
java_ver="$(java -version 2>&1 | head -n 1 || true)"

clean_native() {
  local proj="$1"
  rm -rf "${proj}/.jpm/out" "${proj}/.jpm/work" "${proj}/jpm.lock.yaml"
}

# Maven build runs through JPM's Maven bridge which writes to .jpm/out as well.
clean_maven() {
  local proj="$1"
  rm -rf "${proj}/.jpm/out" "${proj}/.jpm/maven" "${proj}/.jpm/logs"
}

# time_ms CMD...
# Prints integer ms to stdout.
time_ms() {
  local start_ns end_ns
  start_ns="$(date +%s%N)"
  "$@" >/dev/null
  end_ns="$(date +%s%N)"
  echo $(( (end_ns - start_ns) / 1000000 ))
}

mean_ms() {
  local sum=0
  local n=0
  while read -r x; do
    [[ -z "${x}" ]] && continue
    sum=$((sum + x))
    n=$((n + 1))
  done
  if [[ ${n} -eq 0 ]]; then
    echo 0
  else
    echo $((sum / n))
  fi
}

median_ms() {
  # expects a file with one integer per line
  local f="$1"
  local n
  n=$(wc -l <"${f}" | tr -d ' ')
  if [[ ${n} -eq 0 ]]; then
    echo 0
    return
  fi
  # sort numeric
  local sorted
  sorted=$(mktemp)
  sort -n "${f}" >"${sorted}"
  if (( n % 2 == 1 )); then
    # odd
    sed -n "$(( (n+1)/2 ))p" "${sorted}"
  else
    # even: average middle two
    local a b
    a=$(sed -n "$(( n/2 ))p" "${sorted}")
    b=$(sed -n "$(( n/2 + 1 ))p" "${sorted}")
    echo $(( (a + b) / 2 ))
  fi
  rm -f "${sorted}"
}

run_native() {
  local proj="$1"
  local out_file="$2"
  local times_file="$3"
  : >"${out_file}"
  : >"${times_file}"

  for i in $(seq 1 "${RUNS}"); do
    clean_native "${proj}"
    # Capture output for the first run (useful for demo / debugging)
    if [[ ${i} -eq 1 ]]; then
      echo "[bench] native run ${i}/${RUNS}" >>"${out_file}"
      ("${JPM_BIN}" build -v "${proj}" 2>&1 | tee -a "${out_file}") >/dev/null
      # Extract duration line if present, otherwise fallback to external timing.
      # The CLI prints: "  duration 123ms" or "  duration 1.23s".
      local dur
      dur=$(grep -E "^  duration " "${out_file}" | tail -n 1 | awk '{print $2}' || true)
      if [[ -n "${dur}" ]]; then
        # parse go-style duration: supports ms and s
        if [[ "${dur}" == *ms ]]; then
          python3 - <<PY >>"${times_file}"
import math
s='${dur%ms}'
print(int(float(s)))
PY
        elif [[ "${dur}" == *s ]]; then
          # seconds to ms (3 decimals max)
          python3 - <<PY >>"${times_file}"
import sys
s='${dur%s}'
print(int(float(s)*1000))
PY
        else
          # unknown format
          time_ms "${JPM_BIN}" build "${proj}" >>"${times_file}"
        fi
      else
        time_ms "${JPM_BIN}" build "${proj}" >>"${times_file}"
      fi
    else
      local t
      t=$(time_ms "${JPM_BIN}" build "${proj}")
      echo "${t}" >>"${times_file}"
    fi
  done
}

run_maven_bridge() {
  local proj="$1"
  local out_file="$2"
  local times_file="$3"
  : >"${out_file}"
  : >"${times_file}"

  if ! command -v mvn >/dev/null 2>&1; then
    echo "[bench] mvn not found; skipping Maven benchmark." >>"${out_file}"
    return 0
  fi

  # Temporarily force engine=maven without modifying jpm.yaml on disk.
  # We do it by copying project_test to a temp dir and editing it.
  local tmp
  tmp=$(mktemp -d)
  cp -a "${proj}/." "${tmp}/"
  # Ensure YAML has engine: maven
  if grep -q "^engine:" "${tmp}/jpm.yaml"; then
    sed -i 's/^engine:.*/engine: maven/' "${tmp}/jpm.yaml"
  else
    printf '\nengine: maven\n' >>"${tmp}/jpm.yaml"
  fi

  for i in $(seq 1 "${RUNS}"); do
    rm -rf "${tmp}/.jpm/out" "${tmp}/.jpm/maven" "${tmp}/.jpm/logs"

    if [[ ${i} -eq 1 ]]; then
      echo "[bench] maven run ${i}/${RUNS}" >>"${out_file}"
      ("${JPM_BIN}" build -v "${tmp}" 2>&1 | tee -a "${out_file}") >/dev/null
      # No structured duration from Maven path; use external timing for consistency.
      local t
      t=$(time_ms "${JPM_BIN}" build "${tmp}")
      echo "${t}" >>"${times_file}"
    else
      local t
      t=$(time_ms "${JPM_BIN}" build "${tmp}")
      echo "${t}" >>"${times_file}"
    fi
  done

  rm -rf "${tmp}"
}

summarize_times() {
  local times_file="$1"
  local mean median warm_mean warm_median
  mean=$(mean_ms <"${times_file}")
  median=$(median_ms "${times_file}")

  warm_mean=0
  warm_median=0
  if [[ $(wc -l <"${times_file}") -gt 1 ]]; then
    tail -n +2 "${times_file}" | mean_ms >/tmp/.warm_mean
    warm_mean=$(cat /tmp/.warm_mean)
    rm -f /tmp/.warm_mean

    local warm_file
    warm_file=$(mktemp)
    tail -n +2 "${times_file}" >"${warm_file}"
    warm_median=$(median_ms "${warm_file}")
    rm -f "${warm_file}"
  fi

  printf "%s %s %s %s" "${mean}" "${median}" "${warm_mean}" "${warm_median}"
}

bench_one() {
  local label="$1"
  local proj="$2"

  local native_log native_times maven_log maven_times
  native_log="${TS_DIR}/${label}.native.log"
  native_times="${TS_DIR}/${label}.native.times"
  maven_log="${TS_DIR}/${label}.maven.log"
  maven_times="${TS_DIR}/${label}.maven.times"

  run_native "${proj}" "${native_log}" "${native_times}"
  run_maven_bridge "${proj}" "${maven_log}" "${maven_times}"

  local n_mean n_median n_wmean n_wmedian
  read -r n_mean n_median n_wmean n_wmedian <<<"$(summarize_times "${native_times}")"

  local m_status="skipped" m_mean=0 m_median=0 m_wmean=0 m_wmedian=0
  if command -v mvn >/dev/null 2>&1; then
    m_status="ok"
    if [[ -f "${maven_times}" ]] && [[ $(wc -l <"${maven_times}") -gt 0 ]]; then
      read -r m_mean m_median m_wmean m_wmedian <<<"$(summarize_times "${maven_times}")"
    fi
  fi

  cat <<JSON
    "${label}": {
      "project": "${proj}",
      "native": {"status":"ok","mean_ms":${n_mean},"median_ms":${n_median},"warm_mean_ms":${n_wmean},"warm_median_ms":${n_wmedian}},
      "maven": {"status":"${m_status}","mean_ms":${m_mean},"median_ms":${m_median},"warm_mean_ms":${m_wmean},"warm_median_ms":${m_wmedian}}
    }
JSON
}

declare -a PROJECTS
if [[ -n "${PROJECT_DIR}" ]]; then
  PROJECTS=("single:${PROJECT_DIR}")
else
  PROJECTS=(
    "small:${ROOT_DIR}/bench/projects/small"
    "medium:${ROOT_DIR}/bench/projects/medium"
    "large:${ROOT_DIR}/bench/projects/large"
  )
fi

json_projects=""
for item in "${PROJECTS[@]}"; do
  label="${item%%:*}"
  proj="${item#*:}"
  if [[ ! -f "${proj}/jpm.yaml" ]]; then
    echo "[bench] missing jpm.yaml for ${label} at ${proj}" >&2
    exit 2
  fi
  chunk="$(bench_one "${label}" "${proj}")"
  if [[ -z "${json_projects}" ]]; then
    json_projects="${chunk}"
  else
    json_projects+=$',\n'"${chunk}"
  fi
done

cat >"${TS_DIR}/bench.json" <<JSON
{
  "timestamp_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "os": "${uname_s}",
  "arch": "${uname_m}",
  "go": "${go_ver}",
  "java": "${java_ver}",
  "maven": "$(mvn -v 2>/dev/null | head -n 1 | sed 's/"/\\"/g' || true)",
  "runs": ${RUNS},
  "projects": {
${json_projects}
  }
}
JSON

# Update latest/ symlinks (copy on filesystems without symlink support)
rm -rf "${LATEST_DIR}" && mkdir -p "${LATEST_DIR}"
cp -a "${TS_DIR}/." "${LATEST_DIR}/"

printf "\n[bench] Results written to:\n  %s\n  %s\n" "${TS_DIR}" "${LATEST_DIR}"
echo "[bench] Done. See bench/results/latest/bench.json for the per-project table."
