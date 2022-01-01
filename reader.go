package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/klauspost/compress/zstd"
)

type Header struct {
	GameVersion       int
	Timestamp         time.Time
	MatchType         MatchType
	Map               Map
	RecordingPlayerID string
	GameMode          GameMode
}

type MatchType int
type GameMode int
type Map int

//go:generate stringer -type=MatchType
//go:generate stringer -type=GameMode
//go:generate stringer -type=Map
const (
	CUSTOM_GAME MatchType = 7
	UNRANKED    MatchType = 12

	BOMB GameMode = 327933806

	KAFE_DOSTOYEVSKY Map = 1378191338
	OREGON           Map = 231702797556
)

var ErrInvalidFile = errors.New("dissect: not a dissect file")
var ErrInvalidLength = errors.New("dissect: received an invalid length of bytes")
var ErrInvalidStringSep = errors.New("dissect: invalid string separator")

var strSep = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

func PrintHead(r io.Reader) error {
	h, err := ReadHeader(r)
	if err != nil {
		return err
	}
	log.Println("Game Version: ", h.GameVersion)
	log.Println("Player ID:    ", h.RecordingPlayerID)
	log.Println("Timestamp:    ", h.Timestamp)
	log.Println("Match Type:   ", h.MatchType)
	log.Println("Game Mode:    ", h.GameMode)
	log.Println("Map:          ", h.Map)
	return nil
}

// Open opens the named compressed file for reading with the dissect format.
func Open(name string) (*io.Reader, error) {
	r, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return NewReader(r)
}

// NewReader decompresses r using zstd and calls ReadHeader
// to validate the dissect format.
func NewReader(r io.Reader) (*io.Reader, error) {
	r, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return ReadHeaderMagic(r)
}

// ReadHeaderMagic reads the header magic of r
// and validates the dissect format.
// If there is an error, it will be of type *ErrInvalidFile.
func ReadHeaderMagic(r io.Reader) (*io.Reader, error) {
	// Checks for the dissect header.
	b, err := readBytes(7, r)
	if err != nil {
		return nil, err
	}
	if string(b[:7]) != "dissect" {
		return nil, ErrInvalidFile
	}
	// Skips to the end of the unknown dissect versioning scheme.
	// Probably will be replaced later when more info is uncovered.
	// We are skipping to the end of the second sequence of 7 0x00 bytes
	// where the string values are stored.
	b = make([]byte, 1)
	n := 0
	t := 0
	for t != 2 {
		len, err := r.Read(b)
		if err != nil {
			return nil, err
		}
		if len != 1 {
			return nil, ErrInvalidFile
		}
		if b[0] == 0x00 {
			if n != 6 {
				n++
			} else {
				n = 0
				t++
			}
		} else if n > 0 {
			n = 0
		}
	}
	return &r, nil
}

// ReadHeaderStr reads the next string in the header of r.
func ReadHeaderStr(r io.Reader) (string, error) {
	b, err := readBytes(1, r)
	if err != nil {
		return "", err
	}
	len := int(b[0])
	b, err = readBytes(7, r)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(b, strSep) {
		return "", ErrInvalidStringSep
	}
	b, err = readBytes(len, r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ReadHeader(r io.Reader) (Header, error) {
	props := make(map[string]string)
	// Loops until the last property is mapped.
	for exists := false; !exists; {
		k, err := ReadHeaderStr(r)
		if err != nil {
			return Header{}, err
		}
		v, err := ReadHeaderStr(r)
		if err != nil {
			return Header{}, err
		}
		props[k] = v
		_, exists = props["teamscore1"]
	}
	h := Header{}
	// Parse game version
	n, err := strconv.Atoi(props["version"])
	if err != nil {
		return h, err
	}
	h.GameVersion = n
	// Parse timestamp
	t, err := time.Parse("2006-01-02-15-04-05", props["datetime"])
	if err != nil {
		return h, err
	}
	h.Timestamp = t
	// Parse match type
	n, err = strconv.Atoi(props["matchtype"])
	if err != nil {
		return h, err
	}
	h.MatchType = MatchType(n)
	// Parse map
	n, err = strconv.Atoi(props["worldid"])
	if err != nil {
		return h, err
	}
	h.Map = Map(n)
	// Parse recording player id
	h.RecordingPlayerID = props["recordingplayerid"]
	// Parse game mode
	n, err = strconv.Atoi(props["gamemodeid"])
	if err != nil {
		return h, err
	}
	h.GameMode = GameMode(n)
	return h, nil
}

func readBytes(n int, r io.Reader) ([]byte, error) {
	b := make([]byte, n)
	len, err := r.Read(b)
	if err != nil {
		return nil, err
	}
	if len != n {
		return nil, ErrInvalidLength
	}
	return b, nil
}
