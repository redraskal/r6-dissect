package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	setup()
	input := viper.GetString("input")
	s, err := os.Stat(input)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	export := viper.GetString("export")
	// Prints match info to console
	if export == "" {
		if head(input, s.IsDir()); err != nil {
			log.Fatal().Err(err).Send()
		}
		return
	}
	// Exports match data to file
	if s.IsDir() {
		err := exportMatch(input, export)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		log.Info().Msg("Output saved.")
		return
	}
	if strings.HasSuffix(export, ".xlsx") {
		log.Fatal().Msg("Dissect will only export a match folder to Excel.")
	}
	// Exports round data to file
	err = exportFile(input, export)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	log.Info().Msg("Output saved.")
}

func setup() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	pflag.StringP("export", "x", "", "specifies the output path (*.json, *.xlsx)")
	pflag.BoolP("debug", "d", false, "sets log level to debug")
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		log.Fatal().Err(err)
	}
	extra := len(pflag.Args())
	if extra < 1 {
		log.Fatal().Msg("Specify a valid match replay file path (*.rec)")
	}
	viper.Set("input", pflag.Args()[0])
	export := viper.GetString("export")
	if len(export) > 0 && !(strings.HasSuffix(export, ".json") || strings.HasSuffix(export, ".xlsx")) {
		log.Fatal().Msg("Specify a valid output path (*.json, *.xlsx)")
	}
	if viper.GetBool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

func head(input string, dir bool) (err error) {
	if dir {
		m, err := NewMatchReader(input)
		if err != nil {
			return err
		}
		defer m.Close()
		m.FirstRound().head()
		return err
	}
	f, err := os.Open(input)
	if err != nil {
		return
	}
	defer f.Close()
	r, err := NewReader(f)
	if err != nil {
		return
	}
	if err := r.ReadPartial(); !Ok(err) {
		return err
	}
	r.head()
	return
}

func exportMatch(input, export string) (err error) {
	m, err := NewMatchReader(input)
	if err != nil {
		return
	}
	defer m.Close()
	if err := m.Read(); !Ok(err) {
		return err
	}
	if strings.HasSuffix(export, ".xlsx") {
		err = m.Export(export)
	} else {
		err = m.ExportJSON(export)
	}
	return
}

func exportFile(input, export string) (err error) {
	f, err := os.Open(input)
	if err != nil {
		return
	}
	defer f.Close()
	r, err := NewReader(f)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	type output struct {
		Header       Header     `json:"header"`
		ActivityFeed []Activity `json:"activityFeed"`
	}
	if err := r.Read(); !Ok(err) {
		return err
	}
	file, err := os.OpenFile(export, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(output{
		r.Header,
		r.Activities,
	})
	return
}
