#!/usr/bin/env sh

set -eu

if [ "$#" -eq 0 ]; then
  set -- likeable
fi

exec "$@"
