#!/usr/bin/env bash
# Print one version's section of CHANGELOG.md, for use as a GitHub release body.
#
# Without this, `goreleaser release` synthesises the body from commit subjects,
# which is how v0.3.0 and v0.1.1 shipped with a raw SHA list headlined by a
# devDependency bump. The prose in CHANGELOG.md is the release note; this is
# what hands it to the release.
set -euo pipefail

usage() {
  echo "Usage: release-notes.sh <version>"
  echo
  echo "  <version>   Release to extract, with or without a leading 'v'."
  echo "              Must match a '## [<version>]' heading in CHANGELOG.md."
}

if [ "$#" -ne 1 ]; then
  usage >&2
  exit 2
fi

case "$1" in
  -h | --help)
    usage
    exit 0
    ;;
esac

# Accept v0.3.2 and 0.3.2 alike: the tag carries the v, the heading does not.
version="${1#v}"

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
changelog="${CHANGELOG_FILE:-$repo_root/CHANGELOG.md}"

if [ ! -f "$changelog" ]; then
  echo "release-notes.sh: no changelog at $changelog" >&2
  exit 1
fi

# Everything after the version's own heading, up to the next '## ' heading.
# The heading itself is dropped: the release page already shows the version.
notes="$(
  awk -v want="## [$version]" '
    index($0, want) == 1 { found = 1; next }
    found && /^## / { exit }
    found { print }
  ' "$changelog"
)"

# An empty section is the dangerous case: goreleaser accepts an empty notes file
# and publishes a release with a blank body, which looks like a successful run.
if [ -z "${notes//[$'\n'[:space:]]/}" ]; then
  echo "release-notes.sh: no notes found for '$version' in $changelog" >&2
  echo "Expected a '## [$version]' heading with content beneath it." >&2
  exit 1
fi

printf '%s\n' "$notes"
