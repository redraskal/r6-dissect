package reader

import (
	"bytes"
	"errors"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog/log"
)

var ErrMissingStaticEOF = errors.New("dissect: missing static data eof")

var staticEOF = []byte{0x6d, 0x88, 0x5f, 0xe5, 0xb7, 0x9c, 0xa3, 0x21}

func (c *Container) ReadStatic() ([]byte, error) {
	if len(c.static) > 0 {
		return c.static, nil
	}
	static, err := readStatic(*c.reader, c.compressed)
	if err != nil {
		return nil, err
	}
	c.static = static
	return static, nil
}

func readStatic(r io.Reader, compressed io.Reader) ([]byte, error) {
	err := skipToStatic(compressed)
	if err != zstd.ErrMagicMismatch {
		return nil, err
	}
	static, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(static) < 8 {
		return static, ErrMissingStaticEOF
	}
	eof := static[len(static)-8:]
	log.Debug().Hex("static_eof", eof).Send()
	if !bytes.Equal(eof, staticEOF) {
		return static, ErrMissingStaticEOF
	}
	return static, nil
}

func skipToStatic(r io.Reader) error {
	b := make([]byte, 20000)
	i := 0
	for {
		_, err := r.Read(b)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Debug().Msgf("static skip incr: %d", i)
			return err
		}
		i++
	}
	return nil
}
