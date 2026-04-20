#!/usr/bin/env bash
# run-plan.sh — execute an entire superpowers plan in ONE fresh Claude
# session. The agent orchestrates the whole milestone, using
# superpowers:subagent-driven-development to dispatch subagents for
# independent tasks (keeps parent context small). Checks off `- [ ]` boxes
# as tasks complete. Final box flip triggers the milestone-close ritual.
#
# If a session dies partway (network, crash, context pressure on very large
# plans), checkboxes persist progress — just re-run and a fresh session
# resumes from the first unchecked box.
#
# Usage:
#   scripts/run-plan.sh <plan-file> [max-retries]
#
# Example:
#   scripts/run-plan.sh docs/superpowers/plans/2026-04-18-m2-cli-polish.md
#   scripts/run-plan.sh docs/superpowers/plans/2026-04-18-m2-cli-polish.md 5
#
# Env vars:
#   SKU_RUNNER_MODEL       — model override (default: inherit claude default)
#   SKU_RUNNER_LOG_DIR     — log dir (default: .planning/runs/<timestamp>)
#   SKU_RUNNER_SLEEP       — seconds between retry attempts (default: 5)

set -euo pipefail

PLAN="${1:-}"
MAX_RETRIES="${2:-5}"
SLEEP_SEC="${SKU_RUNNER_SLEEP:-5}"

if [[ -z "$PLAN" ]]; then
  echo "usage: $0 <plan-file> [max-retries]" >&2
  exit 2
fi
if [[ ! -f "$PLAN" ]]; then
  echo "error: plan file not found: $PLAN" >&2
  exit 2
fi
if [[ ! "$MAX_RETRIES" =~ ^[0-9]+$ ]] || [[ "$MAX_RETRIES" -lt 1 ]]; then
  echo "error: max-retries must be a positive integer (got: $MAX_RETRIES)" >&2
  exit 2
fi
if [[ ! "$SLEEP_SEC" =~ ^[0-9]+$ ]]; then
  echo "error: SKU_RUNNER_SLEEP must be a non-negative integer (got: $SLEEP_SEC)" >&2
  exit 2
fi
if ! command -v claude >/dev/null 2>&1; then
  echo "error: claude CLI not on PATH" >&2
  exit 2
fi

TS="$(date +%Y%m%d-%H%M%S)"
LOG_DIR="${SKU_RUNNER_LOG_DIR:-.planning/runs/$TS}"
mkdir -p "$LOG_DIR"

count_unchecked() { grep -cE '^\s*-\s\[ \]' "$PLAN" || true; }

# Prompt: one session owns the entire plan end-to-end.
read -r -d '' PROMPT <<EOF || true
You are executing a superpowers implementation plan end-to-end in a single
session. Your job is to drive this plan to completion.

Plan file: $PLAN

Invoke the superpowers:subagent-driven-development skill and follow it. That
skill is the required approach for this work — it tells you how to dispatch
subagents for independent tasks so the main session's context stays small
while the plan as a whole gets done.

Execution contract:

1. Read $PLAN end-to-end. Build a mental model of the task graph.
2. Work through tasks in document order. For each unchecked \`- [ ]\`:
   - If the task is substantial and self-contained (writes files, adds tests,
     passes a verification), dispatch a subagent per
     superpowers:subagent-driven-development.
   - If the task is trivial (a one-line tweak, a doc edit, renaming), do it
     inline — spawning a subagent would be overhead.
   - Obey TDD where the plan specifies it: failing test first, implement,
     verify, commit.
   - After the task passes its verification, change its \`- [ ]\` to \`- [x]\`
     in the plan file and commit (plan edit + code + tests together) with a
     conventional message referencing the task.
3. Keep going until every box is \`- [x]\`. Do NOT stop partway because
   "it's a good pause point" — the harness will pause you via context
   pressure if needed.
4. When the LAST \`- [ ]\` has been flipped and committed, perform the
   milestone-close ritual as a FINAL commit:
      a. Determine the next plan: list \`docs/superpowers/plans/*.md\` sorted
         lexicographically, find the one immediately after $PLAN that still
         contains \`- [ ]\`. If none exist, the next state is "v1.0 shipped".
      b. Edit repo-root \`CLAUDE.md\` "## Current milestone" section to point
         at that next plan (use the plan's top "#" heading as the label), or
         write "v1.0 shipped — no open milestone" if none remains. Adjust the
         "Quick path" block to match the next plan if it defines one.
      c. Commit: \`chore(milestone): close <this-plan-basename>, open <next>\`
         (or "close <this-plan-basename>, v1.0 shipped" if none remains).
      d. Tag: \`git tag milestone/<slug>\` where slug is a short form of the
         closed plan (e.g. \`2026-04-18-m1-openrouter-...\` → \`milestone/m1\`).
         Do NOT push the tag.
      e. Fast-forward \`main\` to the just-tagged tip so main always points
         at the latest shipped milestone:
            CURRENT=\$(git rev-parse --abbrev-ref HEAD)
            git checkout main
            git merge --ff-only "\$CURRENT" || {
              echo "WARNING: main is not a fast-forward of \$CURRENT — leaving main alone."
              git checkout "\$CURRENT"
              # continue; do not abort the milestone-close
            }
            git checkout "\$CURRENT"
         Only \`--ff-only\` is allowed. Never merge with conflicts. Never
         push. If main has diverged, leave it alone and warn.
      f. Print "MILESTONE CLOSED: <basename>" to stdout, then exit.

Hard rules:
- Never use --no-verify or skip hooks. If a hook fails, fix the root cause.
- Do not add AI attribution or Co-Authored-By lines anywhere.
- Never push commits or tags.
- NEVER run destructive git commands that touch unrelated state:
  no \`git clean\`, no \`git reset --hard\`, no \`git checkout -- .\`,
  no \`git stash drop\`, no \`git stash -u\` followed by drop, no
  \`git restore\` with broad paths. Use targeted operations only
  (\`git restore <specific-file>\`, \`git stash push <specific paths>\`).
- NEVER delete, move, or modify files outside this plan's scope.
  In particular, NEVER touch \`scripts/\`, \`docs/superpowers/\`, or
  \`.planning/\` — these are runner infrastructure. Treat them as
  read-only. If you think you need to change them, that's a BLOCKED
  condition, not a fix.
- NEVER \`rm -rf\` anything outside files this task explicitly creates
  for cleanup of its own intermediate output. Prefer \`git rm\` for
  removing tracked files within the task's scope.
- If the working tree has untracked or unstaged files unrelated to
  your task, leave them alone. Do not "tidy up" by stashing, cleaning,
  or removing them.
- If you hit a blocker you genuinely cannot resolve (ambiguous spec, external
  dependency unavailable, a failing test whose fix would require a spec
  change), leave that box UNCHECKED, commit a brief "BLOCKED: <task> —
  <reason>" note, and exit. The outer script retries fresh sessions; if no
  session makes progress, it stops so a human can unblock.
- If every box is already \`- [x]\` when you start, print "PLAN COMPLETE"
  and exit without changes (milestone-close should have happened already).
EOF

INITIAL_UNCHECKED="$(count_unchecked)"
echo "[runner] plan:            $PLAN"
echo "[runner] unchecked start: $INITIAL_UNCHECKED"
echo "[runner] max retries:     $MAX_RETRIES"
echo "[runner] log dir:         $LOG_DIR"
echo

if [[ "$INITIAL_UNCHECKED" -eq 0 ]]; then
  echo "[runner] nothing to do — no unchecked boxes."
  exit 0
fi

PREV_UNCHECKED="$INITIAL_UNCHECKED"
for attempt in $(seq 1 "$MAX_RETRIES"); do
  LOG_FILE="$(printf '%s/attempt-%02d.log' "$LOG_DIR" "$attempt")"
  echo "[runner] attempt $attempt/$MAX_RETRIES — unchecked=$PREV_UNCHECKED — log=$LOG_FILE"

  CLAUDE_ARGS=(
    -p "$PROMPT"
    --permission-mode bypassPermissions
  )
  if [[ -n "${SKU_RUNNER_MODEL:-}" ]]; then
    CLAUDE_ARGS+=(--model "$SKU_RUNNER_MODEL")
  fi

  ATTEMPT_START_TS="$(date +%s)"
  ATTEMPT_START_HUMAN="$(date '+%Y-%m-%d %H:%M:%S')"
  echo "[runner] attempt $attempt start: $ATTEMPT_START_HUMAN"

  # Don't abort the bash script on non-zero from claude — we want to inspect
  # progress and decide whether to retry.
  set +e
  claude "${CLAUDE_ARGS[@]}" 2>&1 | tee "$LOG_FILE"
  CLAUDE_EXIT="${PIPESTATUS[0]}"
  set -e

  ATTEMPT_END_TS="$(date +%s)"
  ATTEMPT_END_HUMAN="$(date '+%Y-%m-%d %H:%M:%S')"
  ATTEMPT_DUR=$((ATTEMPT_END_TS - ATTEMPT_START_TS))
  ATTEMPT_DUR_FMT="$(printf '%dm%02ds' $((ATTEMPT_DUR / 60)) $((ATTEMPT_DUR % 60)))"
  echo "[runner] attempt $attempt end:   $ATTEMPT_END_HUMAN (duration: $ATTEMPT_DUR_FMT)"

  NOW_UNCHECKED="$(count_unchecked)"
  echo "[runner] attempt $attempt done — unchecked=$NOW_UNCHECKED (claude exit=$CLAUDE_EXIT)"

  if [[ "$NOW_UNCHECKED" -eq 0 ]]; then
    echo "[runner] PLAN COMPLETE — all boxes checked after $attempt attempt(s)."
    exit 0
  fi

  if [[ "$NOW_UNCHECKED" -ge "$PREV_UNCHECKED" ]]; then
    echo "[runner] no progress this attempt — stopping." >&2
    echo "[runner] inspect $LOG_FILE; resume by re-running once unblocked." >&2
    exit 3
  fi

  PREV_UNCHECKED="$NOW_UNCHECKED"
  echo "[runner] partial progress — retrying with a fresh session in ${SLEEP_SEC}s..."
  sleep "$SLEEP_SEC"
done

echo "[runner] exhausted $MAX_RETRIES retries with $PREV_UNCHECKED unchecked — stopping." >&2
exit 4
