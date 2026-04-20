#!/usr/bin/env bash
# session-stats.sh — summarize Claude Code sessions for this project.
#
# Reads transcripts under ~/.claude/projects/<encoded-cwd>/ and prints a table
# with start time, duration, turn count, token totals (input / output / cache
# read / cache create) per session. Useful after an overnight run to see what
# each plan-author / plan-implement / reconcile session cost.
#
# Usage:
#   scripts/session-stats.sh                    # last 20 sessions for cwd
#   scripts/session-stats.sh --limit 50         # last 50
#   scripts/session-stats.sh --since 2026-04-19 # only sessions started on/after date
#   scripts/session-stats.sh --project-dir <path>
#
# Notes:
#   - Cache-read tokens are billed at ~10% of normal input rate; cache-creation
#     at ~125%. Treat the four columns as separate buckets, not a single total.
#   - Duration is wall-clock between first and last message in the transcript.
#     For interactive sessions this includes idle time.

set -euo pipefail

LIMIT=20
SINCE=""
PROJECT_DIR=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --limit) LIMIT="$2"; shift 2 ;;
    --since) SINCE="$2"; shift 2 ;;
    --project-dir) PROJECT_DIR="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "error: unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [[ -z "$PROJECT_DIR" ]]; then
  # Claude Code encodes the project path by replacing both "/" and "." with "-".
  ENCODED="$(pwd | sed -e 's|/|-|g' -e 's|\.|-|g')"
  PROJECT_DIR="$HOME/.claude/projects/$ENCODED"
fi

if [[ ! -d "$PROJECT_DIR" ]]; then
  echo "error: project dir not found: $PROJECT_DIR" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 not on PATH" >&2
  exit 2
fi

python3 - "$PROJECT_DIR" "$LIMIT" "$SINCE" <<'PY'
import json, os, sys, glob, datetime

project_dir = sys.argv[1]
limit = int(sys.argv[2])
since = sys.argv[3]

files = sorted(glob.glob(os.path.join(project_dir, "*.jsonl")),
               key=os.path.getmtime, reverse=True)

sessions = []
for path in files[:limit * 4]:  # over-fetch in case --since filters most
    sid = os.path.basename(path)[:-6]
    first_ts = last_ts = None
    turns = 0
    in_tok = out_tok = cr_tok = cw_tok = 0
    model = ""
    try:
        with open(path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    d = json.loads(line)
                except json.JSONDecodeError:
                    continue
                ts = d.get("timestamp")
                if ts:
                    if first_ts is None:
                        first_ts = ts
                    last_ts = ts
                msg = d.get("message", {})
                if msg.get("role") == "assistant":
                    turns += 1
                    u = msg.get("usage", {})
                    in_tok += u.get("input_tokens", 0)
                    out_tok += u.get("output_tokens", 0)
                    cr_tok += u.get("cache_read_input_tokens", 0)
                    cw_tok += u.get("cache_creation_input_tokens", 0)
                    model = msg.get("model", model)
    except OSError:
        continue
    if not first_ts:
        continue
    try:
        start = datetime.datetime.fromisoformat(first_ts.replace("Z", "+00:00"))
        end = datetime.datetime.fromisoformat(last_ts.replace("Z", "+00:00"))
    except Exception:
        continue
    if since and start.date().isoformat() < since:
        continue
    dur_s = max(0, int((end - start).total_seconds()))
    sessions.append({
        "id": sid,
        "start": start.astimezone(),
        "dur_s": dur_s,
        "turns": turns,
        "in": in_tok,
        "out": out_tok,
        "cache_r": cr_tok,
        "cache_w": cw_tok,
        "model": model.replace("claude-", "") if model else "?",
    })

sessions = sessions[:limit]

def fmt_n(n):
    if n >= 1_000_000:
        return f"{n / 1_000_000:.2f}M"
    if n >= 1_000:
        return f"{n / 1_000:.1f}k"
    return str(n)

def fmt_dur(s):
    if s >= 3600:
        return f"{s // 3600}h{(s % 3600) // 60:02d}m"
    return f"{s // 60}m{s % 60:02d}s"

if not sessions:
    print("No sessions matched.")
    sys.exit(0)

# Header
hdr = f"{'session':10} {'start (local)':19} {'dur':>8} {'turns':>5} {'input':>8} {'output':>8} {'cache_r':>8} {'cache_w':>8} model"
print(hdr)
print("-" * len(hdr))

tot_in = tot_out = tot_cr = tot_cw = tot_dur = tot_turns = 0
for s in sessions:
    print(f"{s['id'][:8]:10} {s['start'].strftime('%Y-%m-%d %H:%M:%S'):19} "
          f"{fmt_dur(s['dur_s']):>8} {s['turns']:>5} "
          f"{fmt_n(s['in']):>8} {fmt_n(s['out']):>8} "
          f"{fmt_n(s['cache_r']):>8} {fmt_n(s['cache_w']):>8} {s['model']}")
    tot_in += s['in']
    tot_out += s['out']
    tot_cr += s['cache_r']
    tot_cw += s['cache_w']
    tot_dur += s['dur_s']
    tot_turns += s['turns']

print("-" * len(hdr))
print(f"{'TOTAL':10} {len(sessions):>19d} {fmt_dur(tot_dur):>8} {tot_turns:>5} "
      f"{fmt_n(tot_in):>8} {fmt_n(tot_out):>8} "
      f"{fmt_n(tot_cr):>8} {fmt_n(tot_cw):>8} sessions")
PY
