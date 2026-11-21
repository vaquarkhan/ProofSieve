package risk

import "testing"

func TestNegativeKill(t *testing.T) {
	if Score(Input{MutationEnabled: true, MutationKnown: true, KillRatio: -1}) != 20 {
		t.Fatal(Score(Input{MutationEnabled: true, MutationKnown: true, KillRatio: -1}))
	}
	if Score(Input{MaxFiles: 0, CognitiveAdded: -5}) != 0 {
		t.Fatal("clamp")
	}
}
