#!/bin/sh
set -eu
umask 077
docker_certificates=$(mktemp -d)
trap 'rm -rf "$docker_certificates"' EXIT HUP INT TERM
tar -xf /run/secrets/test_docker_config -C "$docker_certificates"
. "$docker_certificates/environment.sh"
export DOCKER_CERT_PATH="$docker_certificates"
"$@"
