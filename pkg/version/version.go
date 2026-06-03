// Package version holds build-time version information for the debi CLI.
package version

// Version is the current version of the debi CLI. It is overridden at build
// time via -ldflags by GoReleaser; "master" is the default for local builds.
var Version = "master"

// Template is the format used by the `debi version` command.
const Template = "debi version %s\n"
