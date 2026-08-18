#!/usr/bin/env bash

set -Eeuo pipefail

environment=${1:?environment is required}
commit_sha=${2:?commit SHA is required}

printf '%s-%s\n' "$environment" "${commit_sha:0:12}"
