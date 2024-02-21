# This example prints operators from a replay.
# python example.py "F:\Replays\Match-2023-06-11_23-16-15-205\Match-2023-06-11_23-16-15-205-R01.rec"

import json
import subprocess
import sys

def parse(input):
	output = subprocess.check_output("r6-dissect", input=input)
	return json.loads(output)

with open(sys.argv[1], "rb") as file:
	replay = parse(file.read())
	for player in replay["players"]:
		print(f"{player['username']} is playing {player['operator']['name']}")
