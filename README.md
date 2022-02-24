# r6-dissect
[![](https://discordapp.com/api/guilds/936737628756271114/widget.png?style=shield)](https://discord.gg/XdEXWQZZAa)
[![Go Reference](https://pkg.go.dev/badge/github.com/redraskal/r6-dissect.svg)](https://pkg.go.dev/github.com/redraskal/r6-dissect)

Match replay API/CLI for Rainbow Six: Siege's Dissect format.

This is a work in progress. I will be using this resource in an upcoming project :eyes:

The data format is subject to change until a stable version is released.

## Current Features
- Parsing match info (Game version, map, gamemode, match type, teams, players)
- Exporting match info to JSON
- Dumping static data to file

## Planned Features
- Player statistics
- Movement packet reading

## CLI Usage
An overview of the file can be printed with the following command:
```
r6-dissect house.rec
```
```
4:31PM INF Version:          Y7S1/6759805
4:31PM INF Recording Player: redraskal [6931694198894320741]
4:31PM INF Match ID:         2eb42844-703c-47cc-a596-a0c7a8506680
4:31PM INF Timestamp:        2022-02-21 11:14:31 -0600 CST
4:31PM INF Match Type:       CUSTOM_GAME_LOCAL
4:31PM INF Game Mode:        BOMB
4:31PM INF Map:              HOUSE
```
You can also write the match info to a JSON file with one of the following commands:
```
r6-dissect "house.rec" -x house.json
r6-dissect "house.rec" -x json house.json
```
```
{
  "header": {
    "gameVersion": "Y7S1",
    "codeVersion": 6759805,
    "timestamp": "2022-02-21T17:14:31Z",
    "matchType": {
      "name": "CUSTOM_GAME_LOCAL",
      "id": 7
    },
    "map": {
      "name": "HOUSE",
      "id": 237873412352
    },
    "recordingPlayerID": "6931694198894320741",
    "additionalTags": "423855620",
    "gamemode": {
      "name": "BOMB",
      "id": 327933806
    },
...
```
#
I would like to credit [draguve](https://github.com/draguve) & other contributors at [draguve/R6-Replays](https://github.com/draguve/R6-Replays) for their additional work on reverse engineering the dissect format.
