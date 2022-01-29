# r6-dissect
[![](https://discordapp.com/api/guilds/936737628756271114/widget.png?style=shield)](https://discord.gg/XdEXWQZZAa)
[![Go Reference](https://pkg.go.dev/badge/github.com/redraskal/r6-dissect.svg)](https://pkg.go.dev/github.com/redraskal/r6-dissect)

Match replay API/CLI for Rainbow Six: Siege's Dissect format.

This is a work in progress. I will be using this resource in an upcoming project :eyes:

The data format is subject to change until a stable version is released.

## Current Features
- Parsing match info (Game version, map, gamemode, match type, teams, players)
- Exporting match info to JSON

## Planned Features
- Player statistics
- Movement packet reading

## CLI Usage
An overview of the file can be printed with the following command:
```
r6-dissect kafe.rec
```
```
7:49PM INF Game Version:     6656289
7:49PM INF Recording Player: JediMasterSoda [8450400598697089250]
7:49PM INF Match ID:         54c3c485-4032-4477-92d5-c006055679b9
7:49PM INF Timestamp:        2021-12-26 18:06:17 -0600 CST
7:49PM INF Match Type:       UNRANKED
7:49PM INF Game Mode:        BOMB
7:49PM INF Map:              KAFE_DOSTOYEVSKY
```
You can also write the match info to a JSON file with one of the following commands:
```
r6-dissect "kafe.rec" -x kafe.json
r6-dissect "kafe.rec" -x json kafe.json
```
```
{
  "header": {
    "gameVersion": 6656289,
    "timestamp": "2021-12-27T00:06:17Z",
    "matchType": {
      "name": "UNRANKED",
      "id": 12
    },
    "map": {
      "name": "KAFE_DOSTOYEVSKY",
      "id": 1378191338
    },
    "recordingPlayerID": "8450400598697089250",
    "additionalTags": "423855620",
    "gamemode": {
      "name": "BOMB",
      "id": 327933806
    },
...
```
#
I would like to credit [draguve](https://github.com/draguve) & other contributors at [draguve/R6-Replays](https://github.com/draguve/R6-Replays) for their additional work on reverse engineering the dissect format.
