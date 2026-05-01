package dissect

type ClutchDetail struct {
	OpponentsAlive int     `json:"opponentsAlive"`
	TimeStarted    float64 `json:"timeStarted"`
	Won            bool    `json:"won"`
	Kills          int     `json:"kills"`
}

type PlayerRoundStats struct {
	Username           string        `json:"username"`
	TeamIndex          int           `json:"-"`
	Score              int           `json:"score"`
	Operator           string        `json:"-"`
	Kills              int           `json:"kills"`
	DBNOs              int           `json:"dbnos"`
	Revives            int           `json:"revives"`
	Died               bool          `json:"died"`
	Assists            int           `json:"assists"`
	Headshots          int           `json:"headshots"`
	HeadshotPercentage float64       `json:"headshotPercentage"`
	OneVx              int           `json:"1vX,omitempty"`
	ClutchDetail       *ClutchDetail `json:"clutchDetail,omitempty"`
}

type PlayerMatchStats struct {
	Username           string  `json:"username"`
	TeamIndex          int     `json:"-"`
	Rounds             int     `json:"rounds"`
	Kills              int     `json:"kills"`
	DBNOs              int     `json:"dbnos"`
	Revives            int     `json:"revives"`
	Deaths             int     `json:"deaths"`
	Assists            int     `json:"assists"`
	Headshots          int     `json:"headshots"`
	HeadshotPercentage float64 `json:"headshotPercentage"`
}

// OpeningKill returns the first player to kill.
func (r *Reader) OpeningKill() MatchUpdate {
	for _, a := range r.MatchFeedback {
		if a.Type == Kill {
			return a
		}
	}
	return MatchUpdate{}
}

// OpeningDeath returns the first player to die (KILL or DEATH activity).
func (r *Reader) OpeningDeath() MatchUpdate {
	for _, a := range r.MatchFeedback {
		if a.Type == Kill || a.Type == Death {
			return a
		}
	}
	return MatchUpdate{}
}

// Trades returns KILL Activity pairs of trades.
func (r *Reader) Trades() [][]MatchUpdate {
	trades := make([][]MatchUpdate, 0)
	var previous = MatchUpdate{}
	for _, a := range r.MatchFeedback {
		var samePlayers = (previous.Target == a.Username || previous.Username == a.Target)
		var withinThreshold = (previous.TimeInSeconds - a.TimeInSeconds) <= 3
		if a.Type == Kill && samePlayers && withinThreshold {
			trades = append(trades, []MatchUpdate{previous, a})
		}
		previous = a
	}
	return trades
}

func (r *Reader) KillsAndDeaths() []MatchUpdate {
	MatchFeedback := make([]MatchUpdate, 0)
	for _, a := range r.MatchFeedback {
		if a.Type == Kill || a.Type == Death {
			MatchFeedback = append(MatchFeedback, a)
		}
	}
	return MatchFeedback
}

func (r *Reader) NumPlayers(team int) int {
	n := 0
	for _, p := range r.Header.Players {
		if p.TeamIndex == team {
			n++
		}
	}
	return n
}

func (r *Reader) PlayerStats() []PlayerRoundStats {
	stats := make([]PlayerRoundStats, 0)
	index := make(map[string]int)
	winningTeamIndex := 0
	if r.Header.Teams[1].Won {
		winningTeamIndex = 1
	}
	for i, p := range r.Header.Players {
		scorePlayer := r.Scoreboard.Players[i]
		stats = append(stats, PlayerRoundStats{
			Username:  p.Username,
			TeamIndex: p.TeamIndex,
			Operator:  p.Operator.String(),
			Assists:   int(scorePlayer.AssistsFromRound),
			Score:     int(scorePlayer.Score),
		})
		index[p.Username] = i
	}
	lastDeath := -1
	for _, a := range r.MatchFeedback {
		i := index[a.Username]
		if a.Type == Kill {
			stats[i].Kills += 1
			if a.Headshot != nil && *a.Headshot {
				stats[i].Headshots += 1
			}
			stats[i].HeadshotPercentage = headshotPercentage(stats[i].Headshots, stats[i].Kills)
			stats[index[a.Target]].Died = true
			lastDeath = index[a.Target]
		} else if a.Type == DBNO {
			stats[i].DBNOs += 1
		} else if a.Type == Revive {
			stats[i].Revives += 1
		} else if a.Type == Death {
			stats[i].Died = true
			lastDeath = i
		}
	}
	// Calculates 1vX
	winnersLeftAlive := make([]int, 0)
	lastDeathWasWinner := false
	for i, p := range r.Header.Players {
		if p.TeamIndex != winningTeamIndex {
			continue
		}
		if !stats[i].Died {
			winnersLeftAlive = append(winnersLeftAlive, i)
		}
		if i == lastDeath {
			lastDeathWasWinner = true
		}
	}
	nWinnersLeftAlive := len(winnersLeftAlive)
	lastWinnerStanding := -1
	if nWinnersLeftAlive == 1 {
		lastWinnerStanding = winnersLeftAlive[0]
	} else if nWinnersLeftAlive == 0 && lastDeathWasWinner {
		lastWinnerStanding = lastDeath
	}
	if lastWinnerStanding > -1 {
		username := stats[lastWinnerStanding].Username
		teamLeft := r.NumPlayers(winningTeamIndex)
		oneVx := 0
		clutchStartTime := float64(0)
		for _, a := range r.MatchFeedback {
			if a.Type == Kill && stats[index[a.Target]].TeamIndex == winningTeamIndex {
				teamLeft--
			} else if a.Type == Death && stats[index[a.Username]].TeamIndex == winningTeamIndex {
				teamLeft--
			} else if a.Type == PlayerLeave && stats[index[a.Username]].TeamIndex == winningTeamIndex {
				teamLeft--
			}
			if teamLeft == 1 && clutchStartTime == 0 {
				clutchStartTime = a.TimeInSeconds
			}
			if a.Username != username {
				continue
			}
			if a.Type == Kill && teamLeft < 2 {
				oneVx++
			}
		}
		for _, s := range stats {
			if s.TeamIndex != winningTeamIndex && !s.Died {
				oneVx++
			}
		}
		stats[lastWinnerStanding].OneVx = oneVx
		if oneVx > 0 {
			losingTeamIndex := 1 - winningTeamIndex
			opponentsAlive := r.NumPlayers(losingTeamIndex)
			for _, a := range r.MatchFeedback {
				// Time counts down in R6 replays (higher = earlier in round).
				// Stop once we reach events at or after clutch start time.
				if a.TimeInSeconds <= clutchStartTime {
					break
				}
				if a.Type == Kill && stats[index[a.Target]].TeamIndex == losingTeamIndex {
					opponentsAlive--
				} else if a.Type == Death && stats[index[a.Username]].TeamIndex == losingTeamIndex {
					opponentsAlive--
				}
			}
			stats[lastWinnerStanding].ClutchDetail = &ClutchDetail{
				OpponentsAlive: opponentsAlive,
				TimeStarted:    clutchStartTime,
				Won:            true,
				Kills:          oneVx,
			}
		}
	}
	return stats
}

func (m *MatchReader) PlayerStats() []PlayerMatchStats {
	stats := make([]PlayerMatchStats, 0)
	index := make(map[string]int)
	for i, r := range m.rounds {
		for _, p := range r.PlayerStats() {
			if len(stats) == 0 || stats[index[p.Username]].Username != p.Username {
				stats = append(stats, PlayerMatchStats{
					Username:  p.Username,
					TeamIndex: p.TeamIndex,
				})
				index[p.Username] = len(index)
			}
			i = index[p.Username]
			stats[i].Rounds += 1
			stats[i].Kills += p.Kills
			stats[i].DBNOs += p.DBNOs
			stats[i].Revives += p.Revives
			if p.Died {
				stats[i].Deaths += 1
			}
			stats[i].Assists += p.Assists
			stats[i].Headshots += p.Headshots
			stats[i].HeadshotPercentage = headshotPercentage(stats[i].Headshots, stats[i].Kills)
		}
	}
	return stats
}

func headshotPercentage(headshots, kills int) float64 {
	if kills == 0 {
		return 0
	}
	return float64(headshots) / float64(kills) * 100
}
