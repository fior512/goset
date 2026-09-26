package version

import "runtime/debug"

// Version build stamp, -ldflags "-X <module>/internal/version.Version=v0.1.0"
var Version = ""

// String build stamp, then go tool module version, then devel
func String() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return "devel"
}
