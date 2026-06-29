#!/bin/sh
set -e

# Map INPUT_TOKEN → GITHUB_TOKEN (overrides the value injected by the runner
# via the action.yml env block, in case the caller supplies a custom token).
if [ -n "${INPUT_TOKEN}" ]; then
  export GITHUB_TOKEN="${INPUT_TOKEN}"
fi

# Map INPUT_WORKING_DIRECTORY → cd into that directory before running.
if [ -n "${INPUT_WORKING_DIRECTORY}" ] && [ "${INPUT_WORKING_DIRECTORY}" != "." ]; then
  cd "${INPUT_WORKING_DIRECTORY}"
fi

# Build the argument list starting with the required subcommand.
ARGS="${INPUT_SUBCOMMAND}"

if [ "${INPUT_DRY_RUN}" = "true" ]; then
  ARGS="${ARGS} --dry-run"
fi

# exec replaces the shell process so signals propagate cleanly.
# shellcheck disable=SC2086  # word-splitting on ARGS is intentional
exec /semrel ${ARGS}
