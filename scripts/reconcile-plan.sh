#!/usr/bin/env bash
# reconcile-plan.sh — bring a superpowers plan's checkboxes in line with
# reality on disk + in git. Runs ONE fresh `claude -p` that audits every
# unchecked `- [ ]` against the actual repo state and flips boxes whose work
# is already done. Does NOT execute any outstanding task — that's run-plan.sh.
#
# Usage:
#   scripts/reconcile-plan.sh <plan-file>
#
# Example:
#   scripts/reconcile-plan.sh docs/superpowers/plans/2026-04-18-m1-openrouter-shard-and-llm-price.md

set -euo pipefail

PLAN="${1:-}"

if [[ -z "$PLAN" ]]; then
  echo "usage: $0 <plan-file>" >&2
  exit 2
fi
if [[ ! -f "$PLAN" ]]; then
  echo "error: plan file not found: $PLAN" >&2
  exit 2
fi
if ! command -v claude >/dev/null 2>&1; then
  echo "error: claude CLI not on PATH" >&2
  exit 2
fi

TS="$(date +%Y%m%d-%H%M%S)"
LOG_DIR="${SKU_RUNNER_LOG_DIR:-.planning/runs/$TS-reconcile}"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/reconcile.log"

count_unchecked() { grep -cE '^\s*-\s\[ \]' "$PLAN" || true; }

read -r -d '' PROMPT <<EOF || true
You are reconciling a superpowers implementation plan against the ACTUAL state
of the repository. Your job is NOT to do outstanding work — only to correct
the plan's checkboxes so a downstream runner can pick up from the real state.

Plan file: $PLAN

Procedure:

1. Read the plan end-to-end so you understand the task graph and deliverables
   (files, commands, tests each task calls for).
2. For each \`- [ ]\` task, audit whether it is ALREADY done:
   - Files the task says to create: do they exist with sensible contents?
   - Commands the task says to make work (build, test, lint, specific sku
     subcommands): run them if cheap, otherwise rely on \`git log\`, grep, and
     file reads.
   - Tests the task says to add: do they exist and pass?
   - \`git log --oneline\` is strong evidence — look for commits whose subject
     or diff clearly covers the task.
3. A task counts as done only if the deliverables exist AND a related commit
   is in history. Partial work (files exist but tests missing, or vice versa)
   stays UNCHECKED — do not mark ambiguous cases complete.
4. For every task you judge complete, change its \`- [ ]\` to \`- [x]\` in place.
   Preserve all other content — indentation, sub-bullets, wording, everything
   else must be byte-identical.
5. When finished, emit a short report to stdout:
      RECONCILE REPORT
      checked: <N>    # newly flipped this run
      skipped: <M>    # unchecked left alone (still TBD)
      ambiguous: <list of task names you weren't sure about, if any>
6. If you made any edits, commit them in ONE commit:
      chore(plan): reconcile <plan-basename> with repo state
   Body: brief list of which tasks you checked off and the evidence for each.
   Do NOT push.
7. If you made zero edits, print "NO CHANGES" and exit without committing.

Hard rules:
- Do NOT execute any outstanding task. If a task is incomplete, leave it alone.
- Do NOT modify any file other than the plan itself.
- Never use --no-verify. No AI attribution or Co-Authored-By lines.
- NEVER run destructive git commands: no \`git clean\`, no
  \`git reset --hard\`, no \`git checkout -- .\`, no \`git stash drop\`,
  no \`git stash -u\` followed by drop, no broad \`git restore\`.
- NEVER touch \`scripts/\`, \`docs/superpowers/specs/\`, or \`.planning/\`.
  Treat them as read-only — they are runner infrastructure.
- If the working tree has untracked files unrelated to this reconcile,
  leave them alone. Do not "tidy up".
- When in doubt, leave it unchecked. False negatives (leaving a done task
  unchecked) are safe — the runner will re-examine. False positives (marking
  an incomplete task done) cause the runner to skip real work.
EOF

BEFORE="$(count_unchecked)"
echo "[reconcile] plan:             $PLAN"
echo "[reconcile] unchecked before: $BEFORE"
echo "[reconcile] log:              $LOG_FILE"
echo

START_TS="$(date +%s)"
START_HUMAN="$(date '+%Y-%m-%d %H:%M:%S')"
echo "[reconcile] start:            $START_HUMAN"

set +e
claude -p "$PROMPT" --permission-mode bypassPermissions 2>&1 | tee "$LOG_FILE"
CLAUDE_EXIT="${PIPESTATUS[0]}"
set -e

END_TS="$(date +%s)"
END_HUMAN="$(date '+%Y-%m-%d %H:%M:%S')"
DUR=$((END_TS - START_TS))
DUR_FMT="$(printf '%dm%02ds' $((DUR / 60)) $((DUR % 60)))"
echo "[reconcile] end:              $END_HUMAN (duration: $DUR_FMT)"

if [[ "$CLAUDE_EXIT" -ne 0 ]]; then
  echo "[reconcile] claude exited $CLAUDE_EXIT — see $LOG_FILE" >&2
  exit 1
fi

AFTER="$(count_unchecked)"
FLIPPED=$((BEFORE - AFTER))
echo
echo "[reconcile] unchecked before: $BEFORE"
echo "[reconcile] unchecked after:  $AFTER"
echo "[reconcile] flipped:          $FLIPPED"
