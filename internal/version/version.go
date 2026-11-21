// Package version is the published ProofSieve release.
package version

// Version is the control-plane release implemented in this module.
const Version = "1.0.0"

// String returns the user-facing version line.
func String() string { return "proofsieve " + Version }
