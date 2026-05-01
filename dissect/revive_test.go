package dissect

import "testing"

func TestDetectRevivesFromScore(t *testing.T) {
	r := &Reader{
		Header: Header{
			Teams: [2]Team{{Role: Attack}, {Role: Defense}},
			Players: []Player{
				{Username: "reviver", TeamIndex: 0},
				{Username: "revived", TeamIndex: 0},
				{Username: "enemy", TeamIndex: 1},
			},
		},
		MatchFeedback: []MatchUpdate{
			{Type: DBNO, Username: "enemy", Target: "revived", Time: "1:09", TimeInSeconds: 69},
			{Type: Kill, Username: "enemy", Target: "revived", Time: "0:00", TimeInSeconds: 0},
		},
		pendingRevives: []scoreReviveCandidate{
			{Username: "reviver", Time: "0:27", TimeInSeconds: 27},
		},
	}

	r.detectRevivesFromScore()

	if len(r.MatchFeedback) != 3 {
		t.Fatalf("expected 3 match feedback events, got %d", len(r.MatchFeedback))
	}
	revive := r.MatchFeedback[1]
	if revive.Type != Revive || revive.Username != "reviver" || revive.Target != "revived" || revive.Time != "0:27" {
		t.Fatalf("unexpected revive event: %+v", revive)
	}
}

func TestReviveClearsPendingDBNOForBleedoutDetection(t *testing.T) {
	r := &Reader{
		MatchFeedback: []MatchUpdate{
			{Type: DBNO, Username: "enemy", Target: "revived", Time: "1:09", TimeInSeconds: 69},
			{Type: Revive, Username: "reviver", Target: "revived", Time: "0:27", TimeInSeconds: 27},
			{Type: Death, Username: "revived", Time: "0:00", TimeInSeconds: 0},
		},
	}

	r.detectBleedouts()

	if r.MatchFeedback[2].BleedOut {
		t.Fatal("expected death after revive not to be marked as bleedout")
	}
}
