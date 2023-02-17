# r6-dissect
[![](https://discordapp.com/api/guilds/936737628756271114/widget.png?style=shield)](https://discord.gg/XdEXWQZZAa)
[![Go Reference](https://pkg.go.dev/badge/github.com/redraskal/r6-dissect.svg)](https://pkg.go.dev/github.com/redraskal/r6-dissect)

Match replay API/CLI for Rainbow Six: Siege's Dissect (.rec) format.

This is a work in progress. I will be using this resource in an upcoming project :eyes:

The data format is subject to change until a stable version is released.

## Current Features
- Parsing match info (Game version, map, gamemode, match type, teams, players)
- Parsing activities with timestamps (Kills, headshots, objective locates, defuser plants/disables, BattlEye bans, DCs)
- Exporting match stats to JSON, Excel, or stdout (JSON)

## Planned Features
- UI alternative
- Track bullet hits/misses
- Track movement packets
- Track other player statistics

### See roadmap at https://github.com/users/redraskal/projects/1/views/1?query=is%3Aopen+sort%3Aupdated-desc.

## CLI Usage
Print a match overview by specifying a match folder or .rec file:
```bash
r6-dissect Match-2023-01-22_01-28-13-135/
# or
r6-dissect Match-2023-01-22_01-28-13-135-R01.rec
```
```
1:15PM INF Version:          Y7S4/7338571
1:15PM INF Recording Player: redraskal [1f63af29-7ebe-48e7-b570-e820632d9565]
1:15PM INF Match ID:         324a1950-a760-4844-a392-1635c5876c0a
1:15PM INF Timestamp:        2023-01-21 19:29:58 -0600 CST
1:15PM INF Match Type:       UNRANKED
1:15PM INF Game Mode:        BOMB
1:15PM INF Map:              CLUB_HOUSE
```
You can export round stats to a JSON file:
```bash
r6-dissect Match-2023-01-22_01-28-13-135-R01.rec -x round.json
```
Example:
```json
{
  "header": {
    "gameVersion": "Y7S4",
    "codeVersion": 7338571,
    "timestamp": "2023-01-22T01:29:58Z",
    "matchType": {
      "name": "UNRANKED",
      "id": 12
    },
    "map": {
      "name": "CLUB_HOUSE",
      "id": 837214085
    },
    "recordingPlayerID": "10079178519866882138",
    "recordingProfileID": "1f63af29-7ebe-48e7-b570-e820632d9565",
    "additionalTags": "423855620",
    "gamemode": {
      "name": "BOMB",
      "id": 327933806
    },
...
  "activityFeed": [
    {
      "type": "KILL",
      "username": "Eilifint.Ve",
      "target": "AnOriginalMango",
      "headshot": false,
      "time": "2:31",
      "timeInSeconds": 151
    },
    {
      "type": "LOCATE_OBJECTIVE",
      "username": "Eilifint.Ve",
      "time": "2:16",
      "timeInSeconds": 136
    },
...
```
Or the entire match:
```bash
r6-dissect Match-2023-01-22_01-28-13-135/ -x match.json
```
Export an Excel spreadsheet by swapping .json with .xlsx.
```bash
r6-dissect Match-2023-01-22_01-28-13-135-R01/ -x match.xlsx
```
Output JSON to the console (stdout) with the following syntax:
```bash
# entire match
r6-dissect Match-2023-01-22_01-28-13-135-R01/ -x stdout
# or single round
r6-dissect Match-2023-01-22_01-28-13-135-R01/Match-2023-01-22_01-28-13-135-R01.rec -x stdout
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
	f, err := os.Open("Match-2022-08-28_23-43-24-133-R01.rec")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r, err := dissect.NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	// Use r.ReadPartial() for faster reads with less data (designed to fill in data gaps in the header)
	// dissect.Ok(err) returns true if the error only pertains to EOF (read was successful)
	if err := r.Read(); !dissect.Ok(err) {
		log.Fatal(err)
	}
	print(r.Header.GameVersion) // Y7S4
}
```

## Exporting match statistics
```go
package main

import (
	"log"

	"github.com/redraskal/r6-dissect/dissect"
)

func main() {
	m, err := dissect.NewMatchReader("MatchReplay/Match-2022-08-28_23-43-24-133/")
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()
	// dissect.Ok(err) returns true if the error only pertains to EOF (read was successful)
	if err := m.Read(); !dissect.Ok(err) {
		log.Fatal(err)
	}
	// You may also try ExportJSON(path string)
	if err := m.Export("match.xlsx"); err != nil {
		log.Fatal(err)
	}
}
```

#
I would like to thank [draguve](https://github.com/draguve) & other contributors at [draguve/R6-Replays](https://github.com/draguve/R6-Replays) for their additional work on reverse engineering the dissect format.
