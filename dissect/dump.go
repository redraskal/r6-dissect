package dissect

import (
	"encoding/hex"
	"github.com/rs/zerolog/log"
	"io"
	"strings"
)

// Dump dumps packet information to w. Packets are separated line-by-line into sections based on game time.
func (r *Reader) Dump(w io.StringWriter) error {
	time := []byte{0x1E, 0xF1, 0x11, 0xAB}
	if r.Header.CodeVersion >= 7408213 { // Y8S1
		time = []byte{0x1F, 0x07, 0xEF, 0xC9}
	}
	timeIndex := 0
	username := []byte{0x22, 0x07, 0x94}
	usernameIndex := 0
	i := 0
	playerIdIndex := make(map[string]string, 0)
	var sb strings.Builder
	_, err := w.WriteString("start:\n---------------\n")
	if err != nil {
		return err
	}
	for {
		b, err := r.Bytes(1)
		if err != nil {
			return err
		}
		if b[0] != time[timeIndex] {
			timeIndex = 0
		} else {
			timeIndex++
			if timeIndex == 4 {
				timeIndex = 0
				if r.Header.CodeVersion >= Y8S1 {
					if err := readTime(r); err != nil {
						return err
					}
				} else {
					if err := readY7Time(r); err != nil {
						return err
					}
				}
				_, err := w.WriteString("\n\n" + r.timeRaw + ":\n---------------\n")
				if err != nil {
					return err
				}
			}
		}
		if b[0] != username[usernameIndex] {
			usernameIndex = 0
		} else {
			usernameIndex++
			if usernameIndex == 3 {
				usernameIndex = 0
				if err = r.Skip(2); err != nil {
					return err
				}
				u, err := r.String()
				if err != nil {
					return err
				}
				if u == "" {
					usernameIndex = 0
					continue
				}
				idIndicator := []byte{0x33, 0xD8, 0x3D, 0x4F, 0x23}
				if err := r.Seek(idIndicator); err != nil {
					return err
				}
				id, err := r.Bytes(4)
				if err != nil {
					return err
				}
				playerIdIndex[strings.ToUpper(hex.EncodeToString(id))] = u
				log.Debug().Str("username", u).Hex("i", id).Msg("found a user!")
			}
		}
		if b[0] == 0x00 {
			i++
		} else {
			i = 0
		}
		_, err = sb.WriteString(strings.ToUpper(hex.EncodeToString(b)))
		if err != nil {
			return err
		}
		if i == 4 {
			out := sb.String()
			if len(strings.Trim(out, "0")) == 0 {
				sb.Reset()
				i = 0
				continue
			}
			out = strings.TrimRight(out, "0")
			if len(out)%2 != 0 {
				out += "0"
			}
			if len(out) > 8 {
				possiblePlayerId := out[len(out)-8:]
				username := playerIdIndex[possiblePlayerId]
				if len(username) > 0 {
					out += " - " + username
				}
			}
			_, err := w.WriteString(out + "\n")
			if err != nil {
				return err
			}
			sb.Reset()
			i = 0
		}
	}
}
