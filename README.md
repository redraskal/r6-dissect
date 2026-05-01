# r6-dissect
[![](https://discordapp.com/api/guilds/936737628756271114/widget.png?style=shield)](https://discord.gg/XdEXWQZZAa)
[![Go Reference](https://pkg.go.dev/badge/github.com/redraskal/r6-dissect.svg)](https://pkg.go.dev/github.com/redraskal/r6-dissect)

Match Replay API/CLI for Rainbow Six: Siege's Dissect (.rec) format.

Supports replays from **Y7S1 through Y11S1** (and counting).

Download the latest version here: https://github.com/redraskal/r6-dissect/releases

## Features
- **Match Info** — Game version, map, gamemode, match type, teams, players, operators
- **Match Feedback** — Kills, headshots, DBNOs, objective locates, defuser plants/disables, BattlEye bans, DCs
- **Scoreboard** — Per-player kills, assists, deaths, and scores with kill/death reconciliation
- **Player Movement** — XYZ position tracking for all 10 players, mapped by entity ID with team assignment
- **Player Rotation** — Yaw and pitch aim angles via quaternion extraction (auto-calibrated per entity)
- **Tactical Events** — Camera destructions, drone destructions, reinforcements, barricades, gadget deployments
- **Output Formats** — JSON or Excel (.xlsx)

### See roadmap at https://github.com/users/redraskal/projects/1.

## CLI Usage
Print a match overview by specifying a match folder or .rec file:
```bash
r6-dissect --info Match-2026-03-26_21-19-17-21932
# or
r6-dissect --info Match-2026-03-26_21-19-17-21932-R01.rec
```
Export round stats to a JSON file:
```bash
r6-dissect Match-2026-03-26_21-19-17-21932-R01.rec -o round.json
```
Example output (abbreviated):
```json
{
  "gameVersion": "Y11S1_Alpha03",
  "codeVersion": 9578674,
  "timestamp": "2026-03-25T23:16:52Z",
  "matchType": { "name": "Ranked", "id": 2 },
  "map": { "name": "Kafe Dostoyevsky", "id": 434715462383 },
  "gamemode": { "name": "Bomb", "id": 327933806 },
  "teams": [
    { "name": "YOUR TEAM", "score": 1, "won": true, "winCondition": "KilledOpponents", "role": "Attack" },
    { "name": "OPPONENTS", "score": 0, "won": false, "role": "Defense" }
  ],
  "players": [
    {
      "id": 1830934665040226621,
      "username": "IanFiftyForty",
      "teamIndex": 0,
      "operator": { "name": "Oryx", "id": 104189664155 }
    }
  ],
  "matchFeedback": [
    { "type": "Kill", "username": "ReithYT", "target": "Ambatakum.", "headshot": false, "time": "1:51", "timeInSeconds": 111 }
  ],
  "movements": [
    {
      "entityId": 160,
      "username": "IanFiftyForty",
      "team": "Attack",
      "positions": [
        { "x": -12.87, "y": -63.22, "z": 0.264, "yaw": -180, "pitch": 5.3 }
      ]
    }
  ]
}
```
Export the entire match:
```bash
r6-dissect Match-2026-03-26_21-19-17-21932 -o match.json
```
Export an Excel spreadsheet:
```bash
r6-dissect Match-2026-03-26_21-19-17-21932 -o match.xlsx
```
Output JSON to stdout:
```bash
# entire match
r6-dissect Match-2026-03-26_21-19-17-21932
# single round
r6-dissect Match-2026-03-26_21-19-17-21932-R01.rec
```

See example outputs in [/examples](https://github.com/redraskal/r6-dissect/tree/main/examples).

## Importing a .rec file
```go
package main

import (
	"log"
	"os"

	"github.com/redraskal/r6-dissect/dissect"
)

func main() {
	f, err := os.Open("Match-2026-03-26_21-19-17-21932-R01.rec")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r, err := dissect.NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	// Use r.ReadPartial() for faster reads with less data
	// dissect.Ok(err) returns true if the error only pertains to EOF (read was successful)
	if err := r.Read(); !dissect.Ok(err) {
		log.Fatal(err)
	}
	print(r.Header.GameVersion) // Y11S1_Alpha03
}
```

## Updating Operator Data
When new operators are added to the game, regenerate the operator roles mapping:
```bash
go generate ./dissect/...
```
This scrapes the current operator list from Ubisoft's website and updates `dissect/operator_roles.go`.

#
I would like to thank [stnokott](https://github.com/stnokott) for their work on r6-dissect, along with [draguve](https://github.com/draguve) & other contributors at [draguve/R6-Replays](https://github.com/draguve/R6-Replays) for their additional reverse engineering work.
