# The README demo: the hello example (examples/hello) dispatching inline
# commands and hello-<sub> executables on PATH by unique prefix.
# Regenerate from the repository root:  vhs docs/vhs/demo.tape
#
# seed.sh builds hello and three external scripts into a fresh
# /tmp/oden.XXXX, with HOME, the cache and PATH inside it, so the recording
# shows no path or tool of yours.

Output assets/demo.gif

Require go

Set Shell "bash"
Set Theme "Catppuccin Mocha"
Set FontSize 14
Set Width 1200
Set Height 560
Set Padding 16
Set WindowBar Colorful
Set Framerate 15
Set TypingSpeed 55ms
Set CursorBlink false

# Off camera: build and seed.
Hide
Type "export D=$(mktemp -d /tmp/oden.XXXX) && docs/vhs/seed.sh $D && . $D/env && clear"
Enter
Wait@90s /^\$$/
Show
Sleep 500ms

# One listing: inline commands and the hello-<sub> executables on PATH.
Type "hello help"
Sleep 300ms
Enter
Sleep 3s

# A unique prefix runs the command; an ambiguous one lists the candidates.
Ctrl+L
Sleep 200ms
Type "hello gr Ada"
Sleep 300ms
Enter
Sleep 1.2s
Type "hello wea Tokyo"
Sleep 300ms
Enter
Sleep 1.2s
Type "hello we"
Sleep 300ms
Enter
Sleep 2s

# hello-web-deploy is web's descendant, so it was no candidate for we;
# it still runs by its own name (and prefix).
Type "hello web-d"
Sleep 300ms
Enter
Sleep 1.2s
Type "hello which wea"
Sleep 300ms
Enter
Sleep 1.5s

# A typo is never run: oden suggests instead.
Type "hello gret"
Sleep 300ms
Enter
Sleep 1.8s

# Inline handlers read files or stdin.
Type "printf 'miso\ndaikon\n' | hello li"
Sleep 300ms
Enter
Sleep 2.5s

Hide
Type "cd / && rm -rf $D"
Enter
Sleep 500ms
