package main

import (
	"encoding/json"
	"github.com/redraskal/r6-dissect/dissect"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var Version = "dev"

type OutputFormat = string

const (
	JSON  OutputFormat = "json"
	Excel OutputFormat = "excel"
)

func main() {
	setup()
	format := viper.GetString("format")
	in, err := viperFileOrDefault("input", os.Stdin, os.O_RDONLY)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	defer in.Close()
	out, err := viperFileOrDefault("output", os.Stdout, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	defer out.Close()
	stat, err := in.Stat()
	if err != nil {
		log.Fatal().Err(err).Send()
	}
	if viper.GetBool("info") {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if err := printHead(in); err != nil {
			log.Fatal().Err(err).Send()
		}
		return
	}
	if viper.GetBool("dump") && stat.IsDir() {
		log.Fatal().Msg("dump requires a replay file input.")
	}
	if viper.GetBool("dump") {
		outBin, err := os.OpenFile(out.Name()+".bin", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		if err != nil {
			log.Fatal().Err(err).Send()
		}
		defer outBin.Close()
		if err := writeRoundDump(in, out, outBin); err != nil {
			log.Fatal().Err(err).Send()
		}
		return
	}
	if stat.IsDir() {
		if err := writeMatch(in, format, out); err != nil {
			log.Fatal().Err(err).Send()
		}
		return
	}
	if format == Excel {
		log.Fatal().Msg("Dissect will only export a match folder to Excel.")
	}
	if err = writeRound(in, out); err != nil {
		log.Fatal().Err(err).Send()
	}
}

func setup() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	pflag.StringP("format", "f", "", "specifies the output format (json, excel)")
	pflag.StringP("output", "o", "", "specifies the output path")
	pflag.BoolP("debug", "d", false, "sets log level to debug")
	pflag.BoolP("dump", "p", false, "dumps packets to the output")
	pflag.Bool("info", false, "prints the replay header")
	pflag.BoolP("version", "v", false, "prints the version")
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		log.Fatal().Err(err)
	}
	if viper.GetBool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
	if viper.GetBool("version") {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Info().Msgf("r6-dissect version: %s", Version)
		log.Info().Msg("https://github.com/redraskal/r6-dissect")
		os.Exit(0)
	}
	extra := len(pflag.Args())
	if extra < 1 && !piped(os.Stdin) {
		log.Fatal().Msg("Specify a valid match replay file/folder path (*.rec files)")
	} else if extra > 0 {
		viper.Set("input", pflag.Args()[0])
	}
	if !viper.IsSet("format") {
		output := viper.GetString("output")
		if strings.HasSuffix(output, ".xlsx") {
			viper.Set("format", "excel")
		} else if strings.HasSuffix(output, ".json") {
			viper.Set("format", "json")
		}
	}
	format := strings.ToLower(viper.GetString("format"))
	if len(format) > 0 && !(format == "json" || format == "excel") {
		log.Fatal().Msg("Specify a valid output format (json, excel)")
	} else if len(format) == 0 {
		viper.Set("format", "json")
	}
}

func printHead(in *os.File) error {
	stat, err := in.Stat()
	if err != nil {
		return err
	}
	if stat.IsDir() {
		m, err := dissect.NewMatchReader(in)
		if err != nil {
			return err
		}
		r, err := m.FirstRound()
		if err != nil {
			return err
		}
		r.Head()
		return nil
	}
	r, err := dissect.NewReader(in)
	if err != nil {
		return err
	}
	if err := r.ReadPartial(); !dissect.Ok(err) {
		return err
	}
	r.Head()
	return nil
}

func writeMatch(in *os.File, format OutputFormat, out io.Writer) error {
	m, err := dissect.NewMatchReader(in)
	if err != nil {
		return err
	}
	if err := m.Read(); !dissect.Ok(err) {
		return err
	}
	if format == Excel {
		return m.WriteExcel(out)
	}
	return m.WriteJSON(out)
}

func writeRound(in io.Reader, out io.Writer) error {
	r, err := dissect.NewReader(in)
	if err != nil {
		return err
	}
	type output struct {
		dissect.Header
		MatchFeedback []dissect.MatchUpdate      `json:"matchFeedback"`
		PlayerStats   []dissect.PlayerRoundStats `json:"stats"`
	}
	if err := r.Read(); !dissect.Ok(err) {
		return err
	}
	encoder := json.NewEncoder(out)
	return encoder.Encode(output{
		r.Header,
		r.MatchFeedback,
		r.PlayerStats(),
	})
}

func writeRoundDump(in io.Reader, out *os.File, outBin *os.File) error {
	r, err := dissect.NewReader(in)
	if err != nil {
		return err
	}
	if _, err := r.Write(outBin); err != nil {
		return err
	}
	if err := r.Dump(out); !dissect.Ok(err) {
		return err
	}
	return nil
}

func piped(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeNamedPipe != 0
}

func viperFileOrDefault(key string, def *os.File, flag int) (*os.File, error) {
	val := viper.GetString(key)
	if len(val) > 0 {
		return os.OpenFile(val, flag, os.ModePerm)
	}
	return def, nil
}
