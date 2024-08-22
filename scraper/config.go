package scraper

const Domain = "https://prod2.seace.gob.pe/seacebus-uiwd-pub/buscadorPublico/buscadorPublico.xhtml"

type ScraperConfig struct {
	Headless  bool
	Domain    string
	StartDate string
	EndDate   string
	Debug     bool // LogFile is a flag to enable logging to a file
}

func NewConfig(headless bool, domain, startDate, endDate string, debug bool) ScraperConfig {
	return ScraperConfig{
		Headless:  headless,
		Domain:    domain,
		StartDate: startDate,
		EndDate:   endDate,
		Debug:     debug,
	}
}
