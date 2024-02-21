// This example prints operators from a replay.
// bun bun.ts "F:\Replays\Match-2023-06-11_23-16-15-205\Match-2023-06-11_23-16-15-205-R01.rec"
// https://bun.sh/docs/runtime/shell

import { argv } from "process";
import { $ } from "bun";

const input = Bun.file(argv[2]);
const result = await $`cat ${input} | r6-dissect`.json();

for (let player of result["players"]) {
	console.log(`${player["username"]} is playing ${player["operator"]["name"]}`);
}
