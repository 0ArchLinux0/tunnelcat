#!/usr/bin/env bash
# setup-fork.sh — re-point this clone's `origin` to the user's GitHub fork.
#
# Use this once, AFTER you have created a fork of tailscale/tailcat under
# your own GitHub account (or a brand-new empty repo) and want this local
# clone to push to it.
#
# Usage:
#   bin/setup-fork.sh <github-username>           # fork of tailscale/tailcat
#   bin/setup-fork.sh <github-username> --new-repo <name>  # brand-new repo
#
# What it does:
#   1. Backs up the current origin URL to bin/.origin.backup
#   2. Sets `origin` to https://github.com/<user>/<repo>.git
#   3. Verifies the new remote is reachable
#   4. Prints a one-line summary so you can confirm
#
# After this runs, you can use `git push` (and `branch-push`) normally.
# The branch-push script in ~/.pi/agent/bin/branch-push still creates
# session branches instead of pushing to main.

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <github-username> [--new-repo <repo-name>]" >&2
  echo "" >&2
  echo "examples:" >&2
  echo "  $0 minjunyeah                  # fork of tailscale/tailcat" >&2
  echo "  $0 minjunyeah --new-repo tunnelcat   # brand-new repo" >&2
  exit 1
fi

USER="$1"
shift

REPO="tailcat"
MODE="fork"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --new-repo)
      MODE="new"
      REPO="$2"
      shift 2
      ;;
    *)
      echo "unknown flag: $1" >&2
      exit 1
      ;;
  esac
done

cd "$(dirname "$0")/.."

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "error: not inside a git repo" >&2
  exit 1
fi

OLD_URL="$(git remote get-url origin 2>/dev/null || echo 'none')"
NEW_URL="https://github.com/${USER}/${REPO}.git"

echo ""
echo "  current origin : ${OLD_URL}"
echo "  new origin     : ${NEW_URL}"
echo "  mode           : ${MODE}"
echo ""

# Backup
BACKUP="bin/.origin.backup"
echo "${OLD_URL}" > "${BACKUP}"
echo "  backed up old origin to ${BACKUP}"
echo ""

# Re-point
git remote set-url origin "${NEW_URL}"
echo "  set origin to ${NEW_URL}"
echo ""

# Verify reachability (don't fail the script if push requires auth —
# this is a HEAD check that works for public repos without credentials).
if git ls-remote --heads origin >/dev/null 2>&1; then
  echo "  verify: remote is reachable ✓"
else
  echo "  verify: remote is NOT reachable from this machine." >&2
  echo "         This is normal if:" >&2
  echo "         - the repo is private and you haven't set up SSH keys" >&2
  echo "         - the repo doesn't exist yet (for --new-repo mode)" >&2
  echo "         The URL is set correctly; you can retry later." >&2
fi

echo ""
echo "done. next steps:"
echo "  - if you used --new-repo: create the empty repo on GitHub first,"
echo "    then run this script again to verify the push"
echo "  - then push your work:  ~/.pi/agent/bin/branch-push \\"
echo "        $(pwd) <short-slug>   # creates a session branch and pushes"
echo ""
