#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly version=$(cat VERSION)
readonly git_sha=$(git rev-parse HEAD)
readonly git_timestamp=$(TZ=UTC git show --quiet --date='format-local:%Y%m%d%H%M%S' --format="%cd")
readonly slug=${version}-${git_timestamp}-${git_sha:0:16}

mkdir bin

echo ""
echo "# Stage riff http-gateway"
echo ""
ko resolve -P -t ${slug} -f config/riff-http-gateway.yaml > bin/riff-http-gateway.yaml
gsutil cp -a public-read bin/riff-http-gateway.yaml gs://projectriff/riff-http-gateway/snapshots/riff-http-gateway-${slug}.yaml
