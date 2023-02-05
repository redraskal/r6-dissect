package dissect

import (
	"strconv"
	"strings"
)

var timeIndicator = []byte{0x1e, 0xf1, 0x11, 0xab}

func (r *DissectReader) readTime() error {
	time, err := r.readString()
	parts := strings.Split(time, ":")
	// Time can show up as a single number in pro league :)
	if len(parts) == 1 {
		seconds, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return err
		}
		r.time = int(seconds)
		r.timeRaw = parts[0]
		return nil
	}
	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	r.time = (minutes * 60) + seconds
	r.timeRaw = time
	return nil
}
