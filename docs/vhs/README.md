# README demo

`demo.tape` records `assets/demo.gif`, the animation under the README header. It drives the hello example (`examples/hello`) beside three external commands on PATH: `hello-web` (the example's own `bin/hello-web`), `hello-weather` and `hello-web-deploy`, two small shell scripts that `seed.sh` writes.

Regenerate from the repository root (needs [VHS](https://github.com/charmbracelet/vhs) and Go; `brew install vhs`):

```sh
vhs docs/vhs/demo.tape
```

It takes under a minute and writes a GIF of about 24 seconds and under 300 KB.

## Your machine stays out of it

Off camera, the tape makes a fresh `/tmp/oden.XXXX` and runs `seed.sh` on it. The script builds `hello` with `-trimpath` (so sources read `hello/main.go`, not your checkout's path) into that directory's `bin`, writes the scripts beside it, and writes an `env` file that sets `PATH` to that `bin` plus `/usr/bin:/bin`, points `HOME` and `XDG_CACHE_HOME` inside the directory and unsets `TUIOS_*`. The help listing therefore shows only the demo's commands and `/tmp` paths. The tape deletes the directory when it ends.

To try the demo commands by hand:

```sh
D=$(mktemp -d /tmp/oden.XXXX) && docs/vhs/seed.sh "$D" && (. "$D/env" && hello help && hello we); rm -rf "$D"
```

## Changing it

Edit the steps between `Show` and `Hide`. Keep the window wide enough for the help listing's source column (`Width 1200` at `FontSize 14`), or its rows wrap. Check a few frames before committing, for example:

```sh
ffmpeg -i assets/demo.gif -vf fps=1/2 /tmp/frame%02d.png
```
