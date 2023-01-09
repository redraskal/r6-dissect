package reader

import (
	"encoding/hex"
	"fmt"
	"github.com/rs/zerolog/log"
	"strings"
)

// HexEventComparison - Debugging tool
type HexEventComparison struct {
	usernames []string
	hex       [][]byte
}

func (c *HexEventComparison) Push(username string, hex []byte) {
	c.usernames = append(c.usernames, username)
	c.hex = append(c.hex, hex)
}

func (c *HexEventComparison) Flush() {
	uniqueColorStart := make([]int, 0)
	uniqueColorEnd := make([]int, 0)
	max := len(c.hex[0])
	unique := false
	common := ""
	for i := 0; i < max; i++ {
		for _, h := range c.hex {
			if len(h) >= max && c.hex[0][i] != h[i] {
				uniqueColorStart = append(uniqueColorStart, i)
				unique = true
			} else {
				if unique {
					uniqueColorEnd = append(uniqueColorEnd, i)
				}
				unique = false
			}
		}
		if unique {
			common += "00"
		} else {
			common += strings.ToUpper(hex.EncodeToString([]byte{c.hex[0][i]}))
		}
	}
	for i, username := range c.usernames {
		str := ""
		h := c.hex[i]
		for _i := 0; _i < len(h); _i++ {
			for _, start := range uniqueColorStart {
				if start == _i {
					// TODO: Figure out why colors do not work
					//str += "\x1B]0;"
					break
				}
			}
			for _, end := range uniqueColorEnd {
				if end == _i {
					//str += "\007"
					break
				}
			}
			str += strings.ToUpper(hex.EncodeToString([]byte{h[_i]}))
		}
		log.Debug().Str("username", fmt.Sprintf("%16s", username)).Str("value", str).Send()
	}
	log.Debug().Str("username", fmt.Sprintf("%16s", "common")).Str("value", common).Send()
}
