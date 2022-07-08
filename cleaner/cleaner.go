package cleaner

import (
	"regexp"
	"strings"

	"github.com/aaaton/golem/v4"
	"github.com/aaaton/golem/v4/dicts/en"
	stemmer "github.com/agonopol/go-stem"
	"github.com/bbalet/stopwords"
)

type Cleaner struct {
	whitespace *regexp.Regexp
	links      *regexp.Regexp
	lemmatizer *golem.Lemmatizer
}

func NewCleaner() *Cleaner {
	l, err := golem.New(en.New())
	if err != nil {
		panic(err)
	}
	return &Cleaner{
		whitespace: regexp.MustCompile("[^a-zA-Z0-9]+"),
		links:      regexp.MustCompile(`(https\S+)`),
		lemmatizer: l,
	}
}

func (c Cleaner) CleanText(text string) []string {
	withoutLinks := c.links.ReplaceAllString(text, "")
	alphabeticalOnly := c.whitespace.ReplaceAllString(withoutLinks, "")
	withoutStopWords := stopwords.CleanString(alphabeticalOnly, "en", false)
	split := strings.Split(withoutStopWords, " ")
	for i := range split {
		split[i] = c.lemmatizer.Lemma(
			string(
				stemmer.Stem(
					[]byte(split[i]),
				),
			),
		)
	}
	return split
}
