package main

import (
	"encoding/json"
	"os"

	"github.com/redraskal/r6-dissect/reader"
	"github.com/redraskal/r6-dissect/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	setupFlags()
	setupLogging()
	r, err := reader.Open(viper.GetString("input"))
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	if viper.GetString("export") != "" {
		type output struct {
			Header types.Header `json:"header"`
		}
		h, err := reader.ReadHeader(*r)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		file, _ := os.OpenFile(viper.GetString("output"), os.O_CREATE|os.O_TRUNC, os.ModePerm)
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.Encode(output{
			h,
		})
		log.Info().Msg("Output saved.")
	} else {
		if err = PrintHead(*r); err != nil {
			log.Fatal().Err(err).Send()
		}
	}
}

func setupFlags() {
	pflag.StringP("export", "x", "", "specifies the export format (json)")
	pflag.BoolP("debug", "D", false, "sets log level to debug")
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
}

func setupLogging() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if viper.GetBool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
