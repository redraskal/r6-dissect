package test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/redraskal/r6-dissect/dissect"
)

// withFile provides a wrapper for reading and closing a file
func withFile(file string, run func(*os.File, *testing.T)) func(*testing.T) {
	return func(t *testing.T) {
		f, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		run(f, t)
		if err = f.Close(); err != nil {
			t.Errorf(`error closing "%s": %v`, file, err)
		}
	}
}

func readRoundExpected(roundFile string, t *testing.T) *dissect.DissectReader {
	t.Helper()
	jsonFile := roundFile + ".json"
	f, err := os.Open(jsonFile)
	if err != nil {
		t.Fatalf(`could not read "%s": %v`, jsonFile, err)
	}
	r := new(dissect.DissectReader)
	if err = json.NewDecoder(f).Decode(r); err != nil {
		t.Fatalf(`could not decode "%s": %v`, jsonFile, err)
	}
	return r
}

func TestInvalid(t *testing.T) {
	filepath.WalkDir("data/replays/invalid", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			t.Run("invalid replay file "+d.Name(), withFile(path, func(f *os.File, t *testing.T) {
				t.Parallel()
				if _, err := dissect.NewReader(f); err == nil {
					t.Fatal("expected err, got nil")
				}
			}))
		}
		return err
	})
}

// TestNewReader validates data in DissectReader after calling NewReader(), i.e. most header fields
func TestNewReader(t *testing.T) {
	filepath.WalkDir("data/replays/valid", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".rec") {
			t.Run("valid replay file "+d.Name(), withFile(path, func(f *os.File, t *testing.T) {
				t.Parallel()
				gotR, err := dissect.NewReader(f)
				if err != nil {
					t.Fatalf("NewReader(): expected no error, got %v", err)
				}
				// should only be filled by calling .Read(Partial), not when calling NewReader
				if len(gotR.MatchFeedback) > 0 {
					t.Errorf("expected DissectReader.MatchFeedback to be empty, got len=%d", len(gotR.MatchFeedback))
				}
				wantJSON := readRoundExpected(path, t)
				// only specifying fieldsCompare which are filled without calling .Read()
				// would be possible to do dynamically with reflect, but not worth the effort at the moment.
				fieldsCompare := []struct {
					name string
					got  any
					want any
				}{
					{"GameVersion", gotR.Header.GameVersion, wantJSON.Header.GameVersion},
					{"CodeVersion", gotR.Header.CodeVersion, wantJSON.Header.CodeVersion},
					{"Timestamp", gotR.Header.Timestamp, wantJSON.Header.Timestamp},
					{"MatchType", gotR.Header.MatchType, wantJSON.Header.MatchType},
					{"Map", gotR.Header.Map, wantJSON.Header.Map},
					{"RecordingPlayerID", gotR.Header.RecordingPlayerID, wantJSON.Header.RecordingPlayerID},
					{"RecordingProfileID", gotR.Header.RecordingProfileID, wantJSON.Header.RecordingProfileID},
					{"GameMode", gotR.Header.GameMode, wantJSON.Header.GameMode},
					{"RoundsPerMatch", gotR.Header.RoundsPerMatch, wantJSON.Header.RoundsPerMatch},
					{"RoundsPerMatchOvertime", gotR.Header.RoundsPerMatchOvertime, wantJSON.Header.RoundsPerMatchOvertime},
					{"RoundNumber", gotR.Header.RoundNumber, wantJSON.Header.RoundNumber},
					{"OvertimeRoundNumber", gotR.Header.OvertimeRoundNumber, wantJSON.Header.OvertimeRoundNumber},
					{"MatchID", gotR.Header.MatchID, wantJSON.Header.MatchID},
				}

				for _, field := range fieldsCompare {
					if diffs := deep.Equal(field.got, field.want); diffs != nil {
						t.Errorf("Header.%s mismatch (got, want): %v", field.name, diffs)
					}
				}
			}))
		}
		return err
	})
}

// TestDissectReader_ReadPartial validates data in DissectReader after calling .ReadPartial()
func TestDissectReader_ReadPartial(t *testing.T) {
	filepath.WalkDir("data/replays/valid", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".rec") {
			t.Run("valid replay file "+d.Name(), withFile(path, func(f *os.File, t *testing.T) {
				t.Parallel()
				gotR, err := dissect.NewReader(f)
				if err != nil {
					t.Fatalf("NewReader(): expected no error, got %v", err)
				}
				if err = gotR.ReadPartial(); !dissect.Ok(err) {
					t.Fatalf("Read(): expected no error, got %v", err)
				}
				wantJSON := readRoundExpected(path, t)

				// no need to validate header fields as already covered in different test

				// only look at defenders because .ReadPartial() doesn't include attacker swaps and
				// could thus cause false negatives when comparing to the final output JSON.
				gotDefenderPlayers := filterPlayerOps(gotR.Header.Players, dissect.Defense)
				wantDefenderPlayers := filterPlayerOps(wantJSON.Header.Players, dissect.Defense)

				// sort first; we are not interested in the order, but the content.
				// deep.Equal cares about order, so let's sync the order of got and want
				sort.SliceStable(gotDefenderPlayers, func(i, j int) bool {
					return gotDefenderPlayers[i].ID < gotDefenderPlayers[j].ID
				})
				sort.SliceStable(wantDefenderPlayers, func(i, j int) bool {
					return wantDefenderPlayers[i].ID < wantDefenderPlayers[j].ID
				})

				if diffs := deep.Equal(gotDefenderPlayers, wantDefenderPlayers); diffs != nil {
					t.Errorf("Header.Players mismatch (got, want):")
					for _, diff := range diffs {
						t.Error("   " + diff)
					}
				}
			}))
		}
		return err
	})
}

func filterPlayerOps(players []dissect.Player, targetRole dissect.TeamRole) []dissect.Player {
	out := make([]dissect.Player, 0, 5)
	for _, p := range players {
		if p.Operator.Role() == targetRole {
			out = append(out, p)
		}
	}
	return out
}

func TestDissectReader_Read(t *testing.T) {
	filepath.WalkDir("data/replays/valid", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".rec") {
			t.Run("valid replay file "+d.Name(), withFile(path, func(f *os.File, t *testing.T) {
				t.Parallel()
				gotR, err := dissect.NewReader(f)
				if err != nil {
					t.Fatalf("NewReader(): expected no error, got %v", err)
				}
				if err = gotR.Read(); !dissect.Ok(err) {
					t.Fatalf("Read(): expected no error, got %v", err)
				}
				wantJSON := readRoundExpected(path, t)

				if diffs := deep.Equal(gotR.Header, wantJSON.Header); diffs != nil {
					t.Errorf("DissectReader fields mismatch (got, want):")
					for _, diff := range diffs {
						t.Error("   " + diff)
					}
				}
			}))
		}
		return err
	})
}
