# r6-dissect misc scripts

## dump_filter_by_time.sh

This bash script extracts one second of replay from a replay dump into a new text file.

```bash
r6-dissect --dump dump.txt replay.rec

./dump_filter_by_time.sh dump.txt 0:01
# "Data saved to 0_01.txt"
```
