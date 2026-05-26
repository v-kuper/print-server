#!/bin/sh
set -eu

mkdir -p /var/log/uem/agent /var/log/uem/updater

start_uem_agent() {
  if [ ! -x /opt/uem/agent/bin/uema ]; then
    echo "UEMA binary not found at /opt/uem/agent/bin/uema" >&2
    return 1
  fi

  /opt/uem/agent/bin/uema --service=install >/var/log/uem/agent/install.log 2>&1 || true

  if /opt/uem/agent/bin/uema --service=restart >/var/log/uem/agent/restart.log 2>&1; then
    return 0
  fi

  if /opt/uem/agent/bin/uema --service=start >/var/log/uem/agent/start.log 2>&1; then
    return 0
  fi

  /opt/uem/agent/bin/uema >/var/log/uem/agent/runtime.log 2>&1 &
}

start_uem_updater() {
  if [ ! -x /opt/uem/updater/bin/uemu ]; then
    return 0
  fi

  /opt/uem/updater/bin/uemu --service=install >/var/log/uem/updater/install.log 2>&1 || true
  if /opt/uem/updater/bin/uemu --service=restart >/var/log/uem/updater/restart.log 2>&1; then
    return 0
  fi

  if /opt/uem/updater/bin/uemu --service=start >/var/log/uem/updater/start.log 2>&1; then
    return 0
  fi

  /opt/uem/updater/bin/uemu >/var/log/uem/updater/runtime.log 2>&1 &
}

start_uem_agent
start_uem_updater

sleep 1

exec "$@"
