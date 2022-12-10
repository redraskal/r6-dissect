package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/redraskal/r6-dissect/reader"
	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	setupFlags()
	r, err := os.Open(viper.GetString("input"))
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	c, err := reader.NewReader(r)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	if viper.GetString("export") != "" {
		type output struct {
			Header       types.Header     `json:"header"`
			ActivityFeed []types.Activity `json:"activityFeed"`
		}
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		activityFeed, err := c.ReadActivities()
		if err != nil && (err != io.EOF && err != zstd.ErrMagicMismatch) {
			log.Fatal().Err(err).Send()
		}
		file, err := os.OpenFile(viper.GetString("output"), os.O_CREATE|os.O_TRUNC, os.ModePerm)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.Encode(output{
			c.Header,
			activityFeed,
		})
		log.Info().Msg("Output saved.")
	} else {
		PrintHead(c)
		if !viper.GetBool("static") {
			return
		}
		if err = DumpStatic(c); err != nil {
			log.Fatal().Err(err).Send()
		}
	}
}

func setupFlags() {
	pflag.StringP("export", "x", "", "specifies the export format (json)")
	pflag.BoolP("debug", "d", false, "sets log level to debug")
	pflag.BoolP("static", "s", false, "dumps static data to static.bin")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	extra := len(pflag.Args())
	if extra < 1 {
		log.Fatal().Msg("Specify a valid match replay file path (*.rec)")
	}
	viper.Set("input", pflag.Args()[0])
	export := viper.GetString("export")
	if extra > 1 {
		viper.Set("output", pflag.Args()[1])
	} else if export != "" && export != "json" {
		viper.Set("output", export)
		viper.Set("export", "json")
	}
	if export != "" && viper.GetString("output") == "" {
		log.Fatal().Msg("Specify a valid output file path")
	}
	if viper.GetBool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
