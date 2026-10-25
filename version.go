package oden

import (
	"fmt"
	"io"
	"runtime/debug"
)

func writeVersion(w io.Writer, name string, info *debug.BuildInfo) error {
	version, commit, commitTime, modified := "unknown", "unknown", "unknown", "unknown"
	module, goVersion := "unknown", "unknown"
	if info != nil {
		if info.Main.Version != "" {
			version = info.Main.Version
		}
		if info.Main.Path != "" {
			module = info.Main.Path
		}
		if info.GoVersion != "" {
			goVersion = info.GoVersion
		}
		for _, setting := range info.Settings {
			if setting.Value == "" {
				continue
			}
			switch setting.Key {
			case "vcs.revision":
				commit = setting.Value
			case "vcs.time":
				commitTime = setting.Value
			case "vcs.modified":
				modified = setting.Value
			}
		}
	}
	_, err := fmt.Fprintf(w, "%s\nVersion: %s\nCommit: %s\nCommit time: %s\nModified: %s\nGo: %s\nModule: %s\n", name, version, commit, commitTime, modified, goVersion, module)
	return err
}
