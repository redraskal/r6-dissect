package ubi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"golang.org/x/net/html"
)

type Operator struct {
	IsAttacker bool
}

const ubiOperatorsURL string = "https://www.ubisoft.com/de-de/game/rainbow-six/siege/game-info/operators"

func GetOperatorMap() (opNames map[string]Operator, err error) {
	var req *http.Request
	req, err = http.NewRequest("GET", ubiOperatorsURL, nil)
	if err != nil {
		return
	}
	// identify ourselves
	req.Header.Add("User-Agent", "github.com/redraskal/r6-dissect")
	req.Header.Add("Accept", "text/html")
	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	var operatorsJSON []*UbiOperatorJSON
	operatorsJSON, err = parseOperatorHTML(resp.Body)
	if err != nil {
		return
	}

	opNames = map[string]Operator{}
	for _, op := range operatorsJSON {
		opNames[op.Slug] = Operator{
			IsAttacker: op.IsAttacker,
		}
	}
	return
}

func parseOperatorHTML(body io.ReadCloser) ([]*UbiOperatorJSON, error) {
	z := html.NewTokenizer(body)

	inScript := false
	for {
		tt := z.Next()

		// check for error or EOF
		if tt == html.ErrorToken {
			if err := z.Err(); !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("error during HTML parsing: %w", err)
			}
			return nil, errors.New("error: no script tag found in HTML")
		}

		if !inScript && tt == html.StartTagToken && z.Token().Data == "script" {
			inScript = true
		} else if inScript && tt == html.TextToken {
			rawJS := html.UnescapeString(z.Token().Data)
			ubiData, err := parseOperatorJS(rawJS)
			if err != nil {
				return nil, err
			}
			return ubiData.ContentfulGraphQL.OperatorsListContainer.Content, nil
		} else if inScript && tt == html.EndTagToken {
			return nil, errors.New("error: script tag ended without content")
		}
	}
}

var regexJSON = regexp.MustCompile(`(?s)^window\.__PRELOADED_STATE__\s=\s(.+);$`)

func parseOperatorJS(js string) (*ubiOperatorListJSON, error) {
	matches := regexJSON.FindStringSubmatch(js)
	if matches == nil {
		return nil, errors.New("error: regex did not match anything")
	}
	// use first (and only) capture group
	rawJSON := matches[1]
	data := new(ubiOperatorListJSON)
	err := json.Unmarshal([]byte(rawJSON), data)
	return data, err
}

// could also map more things if needed, i.e. operator icon URL
type UbiOperatorJSON struct {
	Slug string `json:"slug"`
	// operatorName
	// operatorIcon.url
	// operatorThumbnail.url
	IsAttacker bool `json:"side"` // Ubisoft again with their funny naming
}

type ubiOperatorListJSON struct {
	ContentfulGraphQL struct {
		OperatorsListContainer struct {
			Content []*UbiOperatorJSON `json:"content"`
		}
	}
}
