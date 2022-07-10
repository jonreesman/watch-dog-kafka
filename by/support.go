package by

import (
	"bufio"
	"log"
	"os"

	"github.com/jonreesman/watch-dog-kafka/cleaner"
	"github.com/navossoc/bayesian"
)

type SpamDetector struct {
	Classifier *bayesian.Classifier
}

func LoadModelFromFiles(spamInput string, hamInput string) SpamDetector {
	var c SpamDetector
	const (
		Ham  bayesian.Class = "Ham"
		Spam bayesian.Class = "Spam"
	)
	var (
		spamText []string
		hamText  []string
	)
	c.Classifier = bayesian.NewClassifier(Ham, Spam)

	cleaner := cleaner.NewCleaner()

	spamText = loadFileToString(spamInput)
	hamText = loadFileToString(hamInput)

	for _, line := range spamText {
		cleaned := cleaner.CleanText(line)
		c.Classifier.Learn(cleaned, Spam)
	}

	for _, line := range hamText {
		cleaned := cleaner.CleanText(line)
		c.Classifier.Learn(cleaned, Ham)
	}
	c.Classifier.ConvertTermsFreqToTfIdf()
	if c.Classifier.DidConvertTfIdf == false {
		log.Fatal("Failed to vectorize model")
	}
	return c
}

func LoadModelFromFile(modelFile string) (c SpamDetector, err error) {
	const (
		Ham  bayesian.Class = "Ham"
		Spam bayesian.Class = "Spam"
	)

	c.Classifier, err = bayesian.NewClassifierFromFile(modelFile)
	if err != nil {
		return c, err
	}
	/*if c.Classifier.IsTfIdf() == false {
		c.Classifier.ConvertTermsFreqToTfIdf()
		if c.Classifier.IsTfIdf() == false {
			log.Fatal("Failed to vectorize model")
		}
	}*/

	return c, nil
}

func loadFileToString(fileName string) []string {
	fileText := make([]string, 0)

	file, err := os.Open(fileName)
	if err != nil {
		log.Panicf("loadFileToString(): failed reading data from file\n")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		fileText = append(fileText, line)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("ImportTickers(): Error in scanning file.")
	}
	return fileText
}
