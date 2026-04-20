#!/usr/bin/env bash
# run-spec.sh — drive the spec to v1.0 end-to-end, fully autonomous.
#
# Alternates two kinds of fresh Claude sessions so each new plan is
# informed by the state of the repo at the time it's written:
#
#   Phase A (implement): if any plan in docs/superpowers/plans/ has
#     unchecked boxes, pick the first in lex order, run reconcile-plan.sh,
#     then run-plan.sh. That session follows
#     superpowers:subagent-driven-development.
#
#   Phase B (author): if all existing plans are fully checked but the spec
#     still has milestones without plan files, invoke author-plans.sh to
#     write exactly the next missing plan (fresh session using
#     superpowers:writing-plans).
#
# The loop exits when there are no unchecked boxes anywhere AND every
# milestone in the spec has a plan file. Stops on first failure; re-running
# resumes from where it left off (checked boxes persist, committed plans
# persist).
#
# Usage:
#   scripts/run-spec.sh [max-retries-per-plan]
#
# Notes:
#   - Plans are ordered by filename. Name them so that lex order = build
#     order (e.g. 2026-04-18-m0-..., 2026-04-18-m3a.1-...).
#   - Safe to run overnight under `caffeinate -is` — sessions are fresh,
#     plan+checkbox state is persisted to git as it goes.

set -euo pipefail

# Trap SIGINT so Ctrl+C produces a clear stop message instead of bleeding
# into the next loop iteration and falsely declaring "Spec shipped".
trap 'echo; echo "[spec] interrupted by user (SIGINT) — stopping."; exit 130' INT

MAX_RETRIES="${1:-5}"
PLANS_DIR="docs/superpowers/plans"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ ! -d "$PLANS_DIR" ]]; then
  echo "error: $PLANS_DIR not found — run from repo root" >&2
  exit 2
fi
for needed in run-plan.sh reconcile-plan.sh author-plans.sh; do
  if [[ ! -x "$SCRIPT_DIR/$needed" ]]; then
    echo "error: $SCRIPT_DIR/$needed must be executable" >&2
    echo "       chmod +x $SCRIPT_DIR/$needed" >&2
    exit 2
  fi
done

count_unchecked_in() { grep -cE '^\s*-\s\[ \]' "$1" 2>/dev/null || true; }

find_next_unchecked_plan() {
  shopt -s nullglob
  local plans=("$PLANS_DIR"/*.md)
  shopt -u nullglob
  if [[ "${#plans[@]}" -eq 0 ]]; then
    return 0
  fi
  local sorted
  IFS=$'\n' sorted=($(printf '%s\n' "${plans[@]}" | sort))
  unset IFS
  for p in "${sorted[@]}"; do
    if [[ "$(count_unchecked_in "$p")" -gt 0 ]]; then
      printf '%s' "$p"
      return 0
    fi
  done
}

count_plans() {
  shopt -s nullglob
  local plans=("$PLANS_DIR"/*.md)
  shopt -u nullglob
  echo "${#plans[@]}"
}

ITER=0
while true; do
  ITER=$((ITER + 1))

  # ---- Phase A: implement the next plan with unchecked boxes --------------
  NEXT_PLAN="$(find_next_unchecked_plan || true)"

  if [[ -n "$NEXT_PLAN" ]]; then
    UNCHK="$(count_unchecked_in "$NEXT_PLAN")"
    echo
    echo "=============================================="
    echo "[spec iter $ITER] IMPLEMENT $NEXT_PLAN ($UNCHK unchecked)"
    echo "=============================================="

    echo "[spec] reconcile phase..."
    "$SCRIPT_DIR/reconcile-plan.sh" "$NEXT_PLAN"

    POST_RECON="$(count_unchecked_in "$NEXT_PLAN")"
    if [[ "$POST_RECON" -gt 0 ]]; then
      echo "[spec] run phase ($POST_RECON unchecked)..."
      set +e
      "$SCRIPT_DIR/run-plan.sh" "$NEXT_PLAN" "$MAX_RETRIES"
      RUN_EXIT=$?
      set -e

      FINAL="$(count_unchecked_in "$NEXT_PLAN")"
      if [[ "$RUN_EXIT" -ne 0 || "$FINAL" -gt 0 ]]; then
        echo "[spec] STOP — $NEXT_PLAN has $FINAL unchecked after run (run-plan exit=$RUN_EXIT)." >&2
        echo "[spec] inspect .planning/runs/ logs, fix blockers, re-run to resume." >&2
        exit 1
      fi
    else
      echo "[spec] plan fully done after reconcile."
    fi

    echo "[spec] DONE $NEXT_PLAN"
    continue
  fi

  # ---- Phase B: no unchecked plans; author the next missing one -----------
  echo
  echo "=============================================="
  echo "[spec iter $ITER] AUTHOR next plan from spec"
  echo "=============================================="
  BEFORE="$(count_plans)"

  set +e
  "$SCRIPT_DIR/author-plans.sh"
  AUTH_EXIT=$?
  set -e

  AFTER="$(count_plans)"

  if [[ "$AUTH_EXIT" -ne 0 ]]; then
    echo "[spec] author-plans.sh exited $AUTH_EXIT — stopping." >&2
    echo "[spec] inspect the most recent log under .planning/runs/*-author/" >&2
    exit "$AUTH_EXIT"
  fi

  if [[ "$AFTER" -le "$BEFORE" ]]; then
    echo
    echo "[spec] No new plan was authored and no unchecked boxes remain."
    echo "[spec] ALL PLANS COMPLETE. Spec shipped."
    exit 0
  fi

  echo "[spec] authored $((AFTER - BEFORE)) new plan(s) — looping to implement."
done
