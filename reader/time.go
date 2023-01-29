package reader

import (
	"strconv"
	"strings"
)

var timeIndicator = []byte{0x1e, 0xf1, 0x11, 0xab}

func (r *DissectReader) readTime() error {
	time, err := r.readString()
	parts := strings.Split(time, ":")
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
