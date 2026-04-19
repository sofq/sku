#!/usr/bin/env bash
# author-plans.sh — author ONE plan file for the next milestone in the spec
# that doesn't yet have one. Invokes a single fresh Claude session that uses
# the superpowers:writing-plans skill, reads the spec, inventories existing
# plans, and writes exactly the next missing plan (lex-sorted by filename).
# Commits the new plan on its own.
#
# Designed to be called in a loop by scripts/run-spec.sh, which alternates
# author sessions with implement sessions so each plan is informed by the
# state of the repo at the time it's written.
#
# Usage:
#   scripts/author-plans.sh
#
# Exit codes:
#   0 — one plan authored and committed, OR no plans were missing
#   non-zero — claude failed; inspect log under .planning/runs/
#
# Env vars:
#   SKU_SPEC_FILE        — spec path (default: docs/superpowers/specs/2026-04-18-sku-design.md)
#   SKU_AUTHOR_MODEL     — model override (default: inherit claude default)
#   SKU_AUTHOR_LOG_DIR   — log dir (default: .planning/runs/<timestamp>-author)

set -euo pipefail

SPEC="${SKU_SPEC_FILE:-docs/superpowers/specs/2026-04-18-sku-design.md}"
PLANS_DIR="docs/superpowers/plans"

if [[ ! -f "$SPEC" ]]; then
  echo "error: spec not found: $SPEC" >&2
  exit 2
fi
if [[ ! -d "$PLANS_DIR" ]]; then
  echo "error: $PLANS_DIR not found — run from repo root" >&2
  exit 2
fi
if ! command -v claude >/dev/null 2>&1; then
  echo "error: claude CLI not on PATH" >&2
  exit 2
fi

TS="$(date +%Y%m%d-%H%M%S)"
LOG_DIR="${SKU_AUTHOR_LOG_DIR:-.planning/runs/$TS-author}"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/author.log"

read -r -d '' PROMPT <<EOF || true
You are authoring ONE superpowers implementation plan for the sku project.

Spec file: $SPEC
Plans dir: $PLANS_DIR

Your job end-to-end in this single session:

1. Invoke the superpowers:writing-plans skill and follow its discipline.
2. Read $SPEC end-to-end. Pay special attention to the
   "Rollout & Milestones" section — it enumerates every milestone through
   v1.0.
3. List existing plan files in $PLANS_DIR. Treat any plan whose file exists
   as ALREADY AUTHORED — do not modify it.
4. Identify the set of milestones in the spec that have no plan file yet.
   From that set, pick the SINGLE next one in build order (i.e. the one
   whose lex-sorted filename would come first).
5. Author ONLY that one plan. Do not write multiple plans this session.
   Follow CLAUDE.md conventions:
   - File name must lex-sort into build order. Use the existing pattern:
     \`2026-04-18-m<N>-<slug>.md\` or for sub-scopes
     \`2026-04-18-m<N>.<sub>-<slug>.md\` (e.g. m3a.1-ec2-rds).
   - Target ≤ ~25 tasks / ~100 checkboxes. If the milestone is too big,
     author only the first sub-scope plan (e.g. m3a.1) and leave the rest
     for later sessions.
   - The plan must be a standalone session-sized chunk runnable by
     scripts/run-plan.sh end-to-end.
6. Commit the new plan on its own with a conventional message such as
   \`plan(m2): author output-polish plan\`. Do not modify anything else.

Hard rules:
- Write exactly ONE new plan file. If you are tempted to write more, stop.
- Do NOT modify any existing plan file.
- Do NOT modify the spec.
- Do NOT add AI attribution or Co-Authored-By lines.
- Do NOT push commits or tags.
- Never use --no-verify or skip hooks. If a hook fails, fix the root cause.
- NEVER run destructive git commands: no \`git clean\`, no
  \`git reset --hard\`, no \`git checkout -- .\`, no \`git stash drop\`,
  no \`git stash -u\` followed by drop, no broad \`git restore\`.
- NEVER touch \`scripts/\`, \`docs/superpowers/specs/\`, or \`.planning/\`.
  Treat them as read-only — they are runner infrastructure.
- If the working tree has untracked files unrelated to authoring this
  plan, leave them alone. Do not "tidy up".
- If every milestone in the spec already has a plan file, print
  "AUTHOR COMPLETE — 0 plan(s) written" and exit cleanly without changes.

When done, print exactly one of:
  "AUTHOR COMPLETE — 1 plan(s) written: <path>"
  "AUTHOR COMPLETE — 0 plan(s) written"
and exit.
EOF

echo "[author] spec:      $SPEC"
echo "[author] plans dir: $PLANS_DIR"
echo "[author] log:       $LOG_FILE"
echo

CLAUDE_ARGS=(
  -p "$PROMPT"
  --permission-mode bypassPermissions
)
if [[ -n "${SKU_AUTHOR_MODEL:-}" ]]; then
  CLAUDE_ARGS+=(--model "$SKU_AUTHOR_MODEL")
fi

set +e
claude "${CLAUDE_ARGS[@]}" 2>&1 | tee "$LOG_FILE"
EXIT="${PIPESTATUS[0]}"
set -e

if [[ "$EXIT" -ne 0 ]]; then
  echo "[author] claude exited $EXIT — inspect $LOG_FILE" >&2
  exit "$EXIT"
fi

# Sentinel check: a clean run MUST end with "AUTHOR COMPLETE — N plan(s) written".
# Absence means the session was interrupted, crashed, or returned malformed
# output even though the exit code was 0 (e.g. SIGINT swallowed by the pipe).
# Treat that as a hard failure so run-spec.sh doesn't proceed on bad data.
if ! grep -qE '^AUTHOR COMPLETE — [0-9]+ plan\(s\) written' "$LOG_FILE"; then
  echo "[author] no AUTHOR COMPLETE sentinel in $LOG_FILE — session did not finish cleanly." >&2
  echo "[author] possible causes: SIGINT, crash, model truncation, prompt drift." >&2
  exit 3
fi

echo "[author] done."
