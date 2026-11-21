// Package risk scores a pull request from diff size, complexity, critical paths, and mutation.
package risk

import "math"

// Input is the normalized evidence for one pull request.
type Input struct {
	Files           int
	MaxFiles        int
	CognitiveAdded  int
	Critical        bool
	MutationEnabled bool
	MutationKnown   bool
	KillRatio       float64
}

// Score returns R in 0..100 using weights 0.25, 0.20, 0.35, 0.20.
func Score(in Input) int {
	files := 0.0
	if in.MaxFiles > 0 {
		files = math.Min(1, float64(in.Files)/float64(in.MaxFiles))
	}
	cog := math.Min(1, float64(in.CognitiveAdded)/20)
	if cog < 0 {
		cog = 0
	}
	crit := 0.0
	if in.Critical {
		crit = 1
	}
	gap := 0.0
	if in.MutationEnabled {
		if !in.MutationKnown {
			gap = 1
		} else {
			gap = 1 - in.KillRatio
			if gap < 0 {
				gap = 0
			}
			if gap > 1 {
				gap = 1
			}
		}
	}
	raw := 100 * (0.25*files + 0.20*cog + 0.35*crit + 0.20*gap)
	return int(math.Min(100, math.Round(raw)))
}
