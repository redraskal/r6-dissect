package dissect

import (
	"strconv"
	"strings"
)

func (r *DissectReader) readTime() error {
	time, err := r.readString()
	parts := strings.Split(time, ":")
	// Time can show up as a single number in pro league :)
	if len(parts) == 1 {
		seconds, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return err
		}
		r.time = seconds
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
	r.time = float64((minutes * 60) + seconds)
	r.timeRaw = time
	return nil
}
