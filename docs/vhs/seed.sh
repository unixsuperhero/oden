#!/usr/bin/env bash
# seed.sh DIR: build the hello example into DIR/bin beside three external
# hello-<sub> scripts, and write DIR/env, which puts DIR/bin first on a bare
# PATH and points HOME and the XDG directories inside DIR. Nothing reads or
# writes outside DIR.
#
#	D=$(mktemp -d /tmp/oden.XXXX) && docs/vhs/seed.sh "$D" && . "$D/env" && hello help
set -euo pipefail

D=${1:?usage: seed.sh DIR}
D=$(cd "$D" && pwd)
mkdir -p "$D"/{bin,home,cache}
GOWORK=off go build -trimpath -o "$D/bin/hello" ./examples/hello
cp examples/hello/bin/hello-web "$D/bin/"

cat > "$D/bin/hello-weather" <<'SH'
#!/bin/sh
# An external command: any hello-<sub> on PATH is hello's command <sub>.
echo "weather in ${1:-Osaka}: sunny, 21C"
SH
cat > "$D/bin/hello-web-deploy" <<'SH'
#!/bin/sh
# A descendant of hello-web: it never makes a prefix of web ambiguous.
echo "deploying ${1:-site} (parent $ODEN_PARENT)"
SH
chmod +x "$D"/bin/*

cat > "$D/env" <<ENV
unset TUIOS_ENV TUIOS_SESSION TUIOS_PANE_ID TUIOS_SOCKET
export HOME=$D/home XDG_CACHE_HOME=$D/cache
export PATH=$D/bin:/usr/bin:/bin
export PS1='$ '
cd "$D"
ENV
