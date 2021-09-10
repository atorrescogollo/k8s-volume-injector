#!/bin/sh

set -x
/k8s-volume-injector -config ${CONFIG_FILE:-/config/config.yaml}
