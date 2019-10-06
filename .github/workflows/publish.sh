#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly version=$(cat VERSION)
readonly git_sha=$(git rev-parse HEAD)
readonly git_timestamp=$(TZ=UTC git show --quiet --date='format-local:%Y%m%d%H%M%S' --format="%cd")
readonly slug=${version}-${git_timestamp}-${git_sha:0:16}
readonly git_branch=${1:11} # drop 'refs/head/' prefix

echo "Publishing riff http-gateway"

gsutil cp -a public-read gs://projectriff/riff-http-gateway/snapshots/riff-http-gateway-${slug}.yaml gs://projectriff/riff-http-gateway/riff-http-gateway-${version}.yaml

echo "Publishing version references"
gsutil -h 'Content-Type: text/plain' -h 'Cache-Control: private' cp -a public-read <(echo "${slug}") gs://projectriff/riff-http-gateway/snapshots/versions/${git_branch}
gsutil -h 'Content-Type: text/plain' -h 'Cache-Control: private' cp -a public-read <(echo "${slug}") gs://projectriff/riff-http-gateway/snapshots/versions/${version}
if [[ ${version} != *"-snapshot" ]] ; then
  gsutil -h 'Content-Type: text/plain' -h 'Cache-Control: private' cp -a public-read <(echo "${version}") gs://projectriff/riff-http-gateway/versions/releases/${git_branch}
fi
