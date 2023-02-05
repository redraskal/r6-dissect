package main

import (
	"encoding/hex"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"strings"
)

// hexEventComparison - Debugging tool
type hexEventComparison struct {
	usernames []string
	hex       [][]byte
}

func (c *hexEventComparison) Push(username string, hex []byte) {
	c.usernames = append(c.usernames, username)
	c.hex = append(c.hex, hex)
}

func (c *hexEventComparison) Flush() {
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

var headerFont = &excelize.Font{
	Family: "Arial",
	Size:   24,
}

type excelCompass struct {
	f        *excelize.File
	s        string
	row, col int
}

func newExcelCompass(f *excelize.File, sheet string) *excelCompass {
	return &excelCompass{
		f: f,
		s: sheet,
	}
}

func (c *excelCompass) Sheet(sheet string) *excelCompass {
	c.s = sheet
	c.Reset()
	return c
}

func (c *excelCompass) Reset() *excelCompass {
	c.row = 0
	c.col = 0
	return c
}

func (c *excelCompass) Up(n int) *excelCompass {
	c.row -= n
	if c.row < 0 {
		c.row = 0
	}
	return c
}

func (c *excelCompass) Down(n int) *excelCompass {
	c.row += n
	return c
}

func (c *excelCompass) Left(n int) *excelCompass {
	c.col -= n
	if c.col < 0 {
		c.col = 0
	}
	return c
}

func (c *excelCompass) Right(n int) *excelCompass {
	c.col += n
	return c
}

func (c *excelCompass) Cell() string {
	cell, _ := excelize.CoordinatesToCellName(c.col+1, c.row+1, false)
	return cell
}

func (c *excelCompass) Heading(text string) *excelCompass {
	c.f.SetCellRichText(c.s, c.Cell(), []excelize.RichTextRun{
		{
			Text: text,
			Font: headerFont,
		},
	})
	return c
}

func (c *excelCompass) Str(text string) *excelCompass {
	c.f.SetCellStr(c.s, c.Cell(), text)
	return c
}

func (c *excelCompass) Bool(b bool) *excelCompass {
	c.f.SetCellBool(c.s, c.Cell(), b)
	return c
}

func (c *excelCompass) Int(n int) *excelCompass {
	c.f.SetCellInt(c.s, c.Cell(), n)
	return c
}

func (c *excelCompass) Float(n float64, precision int) *excelCompass {
	c.f.SetCellFloat(c.s, c.Cell(), n, precision, 64)
	return c
}
