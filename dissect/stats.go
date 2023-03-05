package dissect

type PlayerRoundStats struct {
	Username           string  `json:"username"`
	TeamIndex          int     `json:"-"`
	Operator           string  `json:"-"`
	Kills              int     `json:"kills"`
	Died               bool    `json:"died"`
	Assists            int     `json:"assists"`
	Headshots          int     `json:"headshots"`
	HeadshotPercentage float64 `json:"headshotPercentage"`
	OneVx              int     `json:"1vX,omitempty"`
}

type PlayerMatchStats struct {
	Username           string  `json:"username"`
	TeamIndex          int     `json:"-"`
	Rounds             int     `json:"rounds"`
	Kills              int     `json:"kills"`
	Deaths             int     `json:"deaths"`
	Assists            int     `json:"assists"`
	Headshots          int     `json:"headshots"`
	HeadshotPercentage float64 `json:"headshotPercentage"`
}

// OpeningKill returns the first player to kill.
func (r *DissectReader) OpeningKill() MatchUpdate {
	for _, a := range r.MatchFeedback {
		if a.Type == KILL {
			return a
		}
	}
	return MatchUpdate{}
}

// OpeningDeath returns the first player to die (KILL or DEATH activity).
func (r *DissectReader) OpeningDeath() MatchUpdate {
	for _, a := range r.MatchFeedback {
		if a.Type == KILL || a.Type == DEATH {
			return a
		}
	}
	return MatchUpdate{}
}

// Trades returns KILL Activity pairs of trades.
func (r *DissectReader) Trades() [][]MatchUpdate {
	trades := make([][]MatchUpdate, 0)
	var previous = MatchUpdate{}
	for _, a := range r.MatchFeedback {
		if a.Type == KILL && previous.Target == a.Username {
			trades = append(trades, []MatchUpdate{previous, a})
		}
	}
	return trades
}

func (r *DissectReader) KillsAndDeaths() []MatchUpdate {
	MatchFeedback := make([]MatchUpdate, 0)
	for _, a := range r.MatchFeedback {
		if a.Type == KILL || a.Type == DEATH {
			MatchFeedback = append(MatchFeedback, a)
		}
	}
	return MatchFeedback
}

func (r *DissectReader) NumPlayers(team int) int {
	n := 0
	for _, p := range r.Header.Players {
		if p.TeamIndex == team {
			n++
		}
	}
	return n
}

func (r *DissectReader) PlayerStats(roundWinTeamIndex int) []PlayerRoundStats {
	stats := make([]PlayerRoundStats, 0)
	index := make(map[string]int)
	for i, p := range r.Header.Players {
		stats = append(stats, PlayerRoundStats{
			Username:  p.Username,
			TeamIndex: p.TeamIndex,
			Operator:  p.RoleName,
		})
		index[p.Username] = i
	}
	lastDeath := -1
	for _, a := range r.MatchFeedback {
		i := index[a.Username]
		if a.Type == KILL {
			stats[i].Kills += 1
			if *a.Headshot {
				stats[i].Headshots += 1
			}
			stats[i].HeadshotPercentage = headshotPercentage(stats[i].Headshots, stats[i].Kills)
			stats[index[a.Target]].Died = true
			lastDeath = index[a.Target]
		} else if a.Type == DEATH {
			stats[i].Died = true
			lastDeath = i
		}
	}
	// Calculates 1vX
	winnersLeftAlive := make([]int, 0)
	lastDeathWasWinner := false
	for i, p := range r.Header.Players {
		if p.TeamIndex != roundWinTeamIndex {
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
		teamLeft := r.NumPlayers(roundWinTeamIndex)
		oneVx := 0
		for _, a := range r.MatchFeedback {
			if a.Type == KILL && stats[index[a.Target]].TeamIndex == roundWinTeamIndex {
				teamLeft--
			} else if a.Type == DEATH && stats[index[a.Username]].TeamIndex == roundWinTeamIndex {
				teamLeft--
			} else if a.Type == PLAYER_LEAVE && stats[index[a.Username]].TeamIndex == roundWinTeamIndex {
				teamLeft--
			}
			if a.Username != username {
				continue
			}
			if a.Type == KILL && teamLeft < 2 {
				oneVx++
			}
		}
		for _, s := range stats {
			if s.TeamIndex != roundWinTeamIndex && !s.Died {
				oneVx++
			}
		}
		stats[lastWinnerStanding].OneVx = oneVx
	}
	return stats
}

func (m *MatchReader) PlayerStats() []PlayerMatchStats {
	stats := make([]PlayerMatchStats, 0)
	index := make(map[string]int)
	for i, r := range m.rounds {
		for _, p := range r.PlayerStats(m.WinningTeamIndex(i)) {
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
