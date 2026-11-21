package risk

import "testing"

func TestScore(t *testing.T) {
	got := Score(Input{Files: 8, MaxFiles: 6, CognitiveAdded: 4, Critical: true, MutationEnabled: true, MutationKnown: true, KillRatio: 0.5})
	if got != 74 {
		t.Fatalf("%d", got)
	}
	if Score(Input{Files: 1, MaxFiles: 0, CognitiveAdded: -1}) != 0 {
		t.Fatal("zeros")
	}
	if Score(Input{MutationEnabled: true, MutationKnown: false}) != 20 {
		t.Fatal("missing mutation")
	}
	if Score(Input{MutationEnabled: true, MutationKnown: true, KillRatio: 2}) != 0 {
		t.Fatal("ratio clamp")
	}
	if Score(Input{Files: 100, MaxFiles: 1, CognitiveAdded: 100, Critical: true, MutationEnabled: true, MutationKnown: false}) != 100 {
		t.Fatal("cap")
	}
}
