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

// GetOperatorMap queries an official Ubisoft resource, mapping operator names to operator metadata
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

	// parse response
	var operatorsJSON []*UbiOperatorJSON
	operatorsJSON, err = parseOperatorHTML(resp.Body)
	if err != nil {
		return
	}

	// convert list to map
	opNames = map[string]Operator{}
	for _, op := range operatorsJSON {
		opNames[op.Slug] = Operator{
			IsAttacker: op.IsAttacker,
		}
	}
	return
}

// parseOperatorHTML extracts Javascript code contained in the Ubisoft response HTML
// and then extracts the contained JS object, parsing it as JSON
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
			// <script> tag found, next html.TextToken should be our JS code
			inScript = true
		} else if inScript && tt == html.TextToken {
			// we expect to find JS code here
			// prepare data by unescaping
			rawJS := html.UnescapeString(z.Token().Data)
			// extract JSON from JS
			ubiData, err := parseOperatorJS(rawJS)
			if err != nil {
				return nil, err
			}
			// return nested JSON data
			return ubiData.ContentfulGraphQL.OperatorsListContainer.Content, nil
		} else if inScript && tt == html.EndTagToken {
			// JS expected, but tag was closed before anything was found
			return nil, errors.New("error: script tag ended without content")
		}
	}
}

// used to extract JSON from JS in HTML
var regexJSON = regexp.MustCompile(`(?s)^window\.__PRELOADED_STATE__\s=\s(.+);$`)

// parseOperatorJS extracts JSON from JS in Ubisoft response using regex
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
