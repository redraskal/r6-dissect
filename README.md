# r6-dissect
[![](https://discordapp.com/api/guilds/936737628756271114/widget.png?style=shield)](https://discord.gg/XdEXWQZZAa)
[![Go Reference](https://pkg.go.dev/badge/github.com/redraskal/r6-dissect.svg)](https://pkg.go.dev/github.com/redraskal/r6-dissect)

Match replay API/CLI for Rainbow Six: Siege's Dissect format.

This is a work in progress. I will be using this resource in an upcoming project :eyes:

The data format is subject to change until a stable version is released.

## Current Features
- Parsing match info (Game version, map, gamemode, match type, teams, players)
- Parsing activities (Kills, headshots, objective locates, BattlEye bans, DCs)
- Exporting match info to JSON
- Dumping static data to file

## Planned Features
- Track plants/disables
- Track bullet hits/misses
- Track movement packets
- Track other player statistics

## CLI Usage
An overview of the file can be printed with the following command:
```
r6-dissect "Match-2022-08-28_23-43-24-133-R01.rec"
```
```
5:37PM INF Version:          Y7S2/7040830
5:37PM INF Recording Player: redraskal [1f63af29-7ebe-48e7-b570-e820632d9565]
5:37PM INF Match ID:         caf4a075-ceb7-406e-ae82-234bef5c00f7
5:37PM INF Timestamp:        2022-08-28 18:45:22 -0500 CDT
5:37PM INF Match Type:       RANKED
5:37PM INF Game Mode:        BOMB
5:37PM INF Map:              KAFE_DOSTOYEVSKY
```
You can also write the match info to a JSON file with one of the following commands:
```
r6-dissect "Match-2022-08-28_23-43-24-133-R01.rec" -x kafe.json
r6-dissect "Match-2022-08-28_23-43-24-133-R01.rec" -x json kafe.json
```
```
{
  "header": {
    "gameVersion": "Y7S2",
    "codeVersion": 7040830,
    "timestamp": "2022-08-28T23:45:22Z",
    "matchType": {
      "name": "RANKED",
      "id": 2
    },
    "map": {
      "name": "KAFE_DOSTOYEVSKY",
      "id": 1378191338
    },
    "recordingPlayerID": "865512328110930947",
    "additionalTags": "423855620",
    "gamemode": {
      "name": "BOMB",
      "id": 327933806
    },
...
  "activityFeed": [
    {
      "type": "KILL",
      "username": "ReithYT",
      "target": "Zonalbuzzard",
      "headshot": true
    },
    {
      "type": "KILL",
      "username": "redraskal",
      "target": "Moyete",
      "headshot": false
    },
    {
      "type": "LOCATE_OBJECTIVE",
      "username": "exoticindo"
    },
...
```
See example outputs in [/examples](https://github.com/redraskal/r6-dissect/tree/main/examples).
#
I would like to credit [draguve](https://github.com/draguve) & other contributors at [draguve/R6-Replays](https://github.com/draguve/R6-Replays) for their additional work on reverse engineering the dissect format.
