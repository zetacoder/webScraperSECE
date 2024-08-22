package main

import (
	"scraper/scraper"
)

func main() {

	config := scraper.NewConfig(false, scraper.Domain, "01/01/2024", "01/10/2024", true)

	scrap, err := scraper.NewScraper(config)
	if err != nil {
		panic(err)
	}
	defer scrap.Close()

	if err := scrap.Search(); err != nil {
		panic(err)
	}

	if err := scrap.GetEntries(); err != nil {
		panic(err)
	}

}
