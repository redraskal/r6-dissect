package dissect

import (
	"encoding/hex"

	"github.com/rs/zerolog/log"
)

type scoreReviveCandidate struct {
	Username      string
	Time          string
	TimeInSeconds float64
}

type Scoreboard struct {
	Players []ScoreboardPlayer
}

type ScoreboardPlayer struct {
	ID               []byte
	Score            uint32
	Assists          uint32
	AssistsFromRound uint32
}

func scoreboardEntryPlayerID(r *Reader) (id [4]byte, ok bool) {
	start := r.offset - 17
	if start < 0 || start+4 > len(r.b) {
		return id, false
	}
	copy(id[:], r.b[start:start+4])
	return id, true
}

func scoreboardEntryHasMarker(r *Reader) bool {
	pos := r.offset - 18
	return pos >= 0 && r.b[pos] == 0x23
}

// this function fixes kills that were previously recorded as elims
func readScoreboardKills(r *Reader) error {
	kills, err := r.Uint32()
	if err != nil {
		return err
	}

	if r.Header.CodeVersion >= Y10S4 {
		return readScoreboardKillsY10S4(r, kills)
	}

	if err := r.Skip(30); err != nil {
		return err
	}
	id, err := r.Bytes(4)
	if err != nil {
		return err
	}
	idx := r.PlayerIndexByID(id)
	if idx != -1 {
		username := r.Header.Players[idx].Username
		r.lastKillerFromScoreboard = username
		log.Warn().
			Str("username", username).
			Uint32("kills", kills).
			Msg("scoreboard_kill")
	}
	return nil
}

func (r *Reader) ensureScoreboardMapping() {
	maxZip := len(r.pendingSBIDs)
	if maxZip > len(r.readPlayerOrder) {
		maxZip = len(r.readPlayerOrder)
	}
	for i := 0; i < maxZip; i++ {
		if _, exists := r.scoreboardIDToPlayer[r.pendingSBIDs[i]]; !exists {
			r.scoreboardIDToPlayer[r.pendingSBIDs[i]] = r.readPlayerOrder[i]
		}
	}

	mappedPlayers := make(map[int]bool)
	for _, idx := range r.scoreboardIDToPlayer {
		mappedPlayers[idx] = true
	}
	for i := maxZip; i < len(r.pendingSBIDs); i++ {
		sbID := r.pendingSBIDs[i]
		if _, exists := r.scoreboardIDToPlayer[sbID]; exists {
			continue
		}
		candidateID := []byte{sbID[0] + 4, sbID[1], sbID[2], sbID[3]}
		idx := r.PlayerIndexByID(candidateID)
		if idx >= 0 && !mappedPlayers[idx] {
			r.scoreboardIDToPlayer[sbID] = idx
			mappedPlayers[idx] = true
		}
	}
}

func readScoreboardKillsY10S4(r *Reader, kills uint32) error {
	sbID, ok := scoreboardEntryPlayerID(r)
	if !ok {
		return nil
	}

	isKnown := false
	for _, pending := range r.pendingSBIDs {
		if pending == sbID {
			isKnown = true
			break
		}
	}

	if !isKnown {
		r.pendingSBIDs = append(r.pendingSBIDs, sbID)
		r.scoreboardInitialKills[sbID] = kills
		log.Debug().
			Str("sbID", hex.EncodeToString(sbID[:])).
			Uint32("kills", kills).
			Int("pendingIdx", len(r.pendingSBIDs)-1).
			Msg("scoreboard_kill_header")
		return nil
	}

	r.ensureScoreboardMapping()

	idx, found := r.scoreboardIDToPlayer[sbID]
	if !found || idx < 0 || idx >= len(r.Header.Players) {
		log.Warn().
			Str("sbID", hex.EncodeToString(sbID[:])).
			Uint32("kills", kills).
			Msg("scoreboard_kill_unknown_player")
		return nil
	}

	username := r.Header.Players[idx].Username
	r.lastKillerFromScoreboard = username
	r.scoreboardFinalKills[idx] = kills
	log.Warn().
		Str("username", username).
		Uint32("kills", kills).
		Str("sbID", hex.EncodeToString(sbID[:])).
		Msg("scoreboard_kill")

	return nil
}

func readScoreboardAssists(r *Reader) error {
	assists, err := r.Uint32()
	if err != nil {
		return err
	}
	if assists == 0 {
		return nil
	}

	if r.Header.CodeVersion >= Y10S4 {
		return readScoreboardAssistsY10S4(r, assists)
	}

	if err = r.Skip(30); err != nil {
		return err
	}
	id, err := r.Bytes(4)
	if err != nil {
		return err
	}
	idx := r.PlayerIndexByID(id)
	username := "N/A"
	if idx != -1 {
		username = r.Header.Players[idx].Username
		r.Scoreboard.Players[idx].Assists = assists
		r.Scoreboard.Players[idx].AssistsFromRound++
	}
	log.Debug().
		Uint32("assists", assists).
		Str("username", username).
		Msg("scoreboard_assists")
	return nil
}

func readScoreboardAssistsY10S4(r *Reader, assists uint32) error {
	if !scoreboardEntryHasMarker(r) {
		return nil
	}
	r.ensureScoreboardMapping()
	sbID, ok := scoreboardEntryPlayerID(r)
	if !ok {
		return nil
	}

	idx, found := r.scoreboardIDToPlayer[sbID]
	username := "N/A"
	if found && idx >= 0 && idx < len(r.Header.Players) {
		username = r.Header.Players[idx].Username
		if idx < len(r.Scoreboard.Players) {
			r.Scoreboard.Players[idx].Assists = assists
			r.Scoreboard.Players[idx].AssistsFromRound++
		}
	}
	log.Debug().
		Uint32("assists", assists).
		Str("username", username).
		Msg("scoreboard_assists")
	return nil
}

func readScoreboardScore(r *Reader) error {
	score, err := r.Uint32()
	if err != nil {
		return err
	}
	if score == 0 {
		return nil
	}

	if r.Header.CodeVersion >= Y10S4 {
		return readScoreboardScoreY10S4(r, score)
	}

	if err = r.Skip(13); err != nil {
		return err
	}
	id, err := r.Bytes(4)
	if err != nil {
		return err
	}
	idx := r.PlayerIndexByID(id)
	username := "N/A"
	if idx != -1 {
		username = r.Header.Players[idx].Username

		prevScore := r.lastPlayerScores[idx]
		if prevScore > 0 && score > prevScore {
			delta := score - prevScore
			if delta == 15 {
				r.recordScoreBasedDestruction(username, "camera", r.time)
			} else if delta == 25 {
				r.recordScoreBasedDestruction(username, "utility", r.time)
			} else if delta == 10 {
				role := r.getPlayerTeamRole(username)
				if role == Defense {
					r.recordScoreBasedDestruction(username, "drone", r.time)
				}
			} else if delta == 20 {
				r.recordScoreBasedDestruction(username, "gadget_deploy", r.time)
			} else if delta == 50 {
				r.pendingRevives = append(r.pendingRevives, scoreReviveCandidate{
					Username:      username,
					Time:          r.timeRaw,
					TimeInSeconds: r.time,
				})
			}
		}
		r.lastPlayerScores[idx] = score

		r.Scoreboard.Players[idx].Score = score
	}
	log.Debug().
		Uint32("score", score).
		Str("username", username).
		Msg("scoreboard_score")
	return nil
}

func readScoreboardScoreY10S4(r *Reader, score uint32) error {
	if !scoreboardEntryHasMarker(r) {
		return nil
	}
	r.ensureScoreboardMapping()
	sbID, ok := scoreboardEntryPlayerID(r)
	if !ok {
		return nil
	}

	idx, found := r.scoreboardIDToPlayer[sbID]
	username := "N/A"
	if found && idx >= 0 && idx < len(r.Header.Players) {
		username = r.Header.Players[idx].Username
		if idx < len(r.Scoreboard.Players) {
			r.Scoreboard.Players[idx].Score = score
		}

		// +100 score bonus identifies defuser planter/disabler
		if r.pendingDefuserPlantIdx >= 0 {
			prevScore := r.lastPlayerScores[idx]
			delta := score - prevScore
			correctTeam := false
			if r.pendingDefuserIsPlant {
				atkIdx := r.getTeamByRole(Attack)
				correctTeam = atkIdx >= 0 && r.Header.Players[idx].TeamIndex == atkIdx
			} else {
				defIdx := r.getTeamByRole(Defense)
				correctTeam = defIdx >= 0 && r.Header.Players[idx].TeamIndex == defIdx
			}
			if prevScore > 0 && delta == 100 && correctTeam {
				r.MatchFeedback[r.pendingDefuserPlantIdx].Username = username
				r.lastDefuserPlayerIndex = idx
				log.Debug().
					Str("username", username).
					Uint32("prevScore", prevScore).
					Uint32("newScore", score).
					Msg("defuser_player_identified_by_score")
				r.pendingDefuserPlantIdx = -1
			}
		}

		prevScore := r.lastPlayerScores[idx]
		if prevScore > 0 && score > prevScore {
			delta := score - prevScore
			if delta == 15 {
				r.recordScoreBasedDestruction(username, "camera", r.time)
			} else if delta == 25 {
				r.recordScoreBasedDestruction(username, "utility", r.time)
			} else if delta == 10 {
				role := r.getPlayerTeamRole(username)
				if role == Defense {
					r.recordScoreBasedDestruction(username, "drone", r.time)
				}
			} else if delta == 20 {
				r.recordScoreBasedDestruction(username, "gadget_deploy", r.time)
			} else if delta == 50 {
				r.pendingRevives = append(r.pendingRevives, scoreReviveCandidate{
					Username:      username,
					Time:          r.timeRaw,
					TimeInSeconds: r.time,
				})
			}
		}

		r.lastPlayerScores[idx] = score
	}
	log.Debug().
		Uint32("score", score).
		Str("username", username).
		Msg("scoreboard_score")
	return nil
}

func (r *Reader) reconcileKillsFromScoreboard() {
	if len(r.scoreboardInitialKills) == 0 {
		return
	}

	r.ensureScoreboardMapping()
	expectedKills := make(map[int]int)
	for sbID, playerIdx := range r.scoreboardIDToPlayer {
		initial, hasInitial := r.scoreboardInitialKills[sbID]
		final, hasFinal := r.scoreboardFinalKills[playerIdx]
		if hasInitial {
			if hasFinal {
				expectedKills[playerIdx] = int(final - initial)
			} else {
				expectedKills[playerIdx] = 0
			}
		}
	}

	actualKills := make(map[int]int)
	for _, u := range r.MatchFeedback {
		if u.Type == Kill {
			idx := r.PlayerIndexByUsername(u.Username)
			if idx >= 0 {
				actualKills[idx]++
			}
		}
	}

	type delta struct {
		idx  int
		diff int
	}
	var overCredited, underCredited []delta
	for playerIdx, expected := range expectedKills {
		actual := actualKills[playerIdx]
		if actual > expected {
			overCredited = append(overCredited, delta{playerIdx, actual - expected})
		} else if actual < expected {
			underCredited = append(underCredited, delta{playerIdx, expected - actual})
		}
	}

	if len(overCredited) == 0 || len(underCredited) == 0 {
		return
	}

	for ui := range underCredited {
		underIdx := underCredited[ui].idx
		if underIdx < 0 || underIdx >= len(r.Header.Players) {
			continue
		}
		underUsername := r.Header.Players[underIdx].Username
		underTeam := r.Header.Players[underIdx].TeamIndex

		for oi := range overCredited {
			if underCredited[ui].diff <= 0 || overCredited[oi].diff <= 0 {
				break
			}
			overIdx := overCredited[oi].idx
			if overIdx < 0 || overIdx >= len(r.Header.Players) {
				continue
			}
			overUsername := r.Header.Players[overIdx].Username

			for i := range r.MatchFeedback {
				if underCredited[ui].diff <= 0 || overCredited[oi].diff <= 0 {
					break
				}
				u := &r.MatchFeedback[i]
				if u.Type != Kill || u.Username != overUsername {
					continue
				}
				targetIdx := r.PlayerIndexByUsername(u.Target)
				if targetIdx < 0 {
					continue
				}
				targetTeam := r.Header.Players[targetIdx].TeamIndex
				if underTeam == targetTeam || underUsername == u.Target {
					continue
				}
				log.Debug().
					Str("original", overUsername).
					Str("corrected", underUsername).
					Str("target", u.Target).
					Msg("scoreboard_reconcile_kill")
				u.Username = underUsername
				underCredited[ui].diff--
				overCredited[oi].diff--
			}
		}
	}
}
