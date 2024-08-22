package scraper

import (
	"encoding/csv"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

type Scraper struct {
	Pw      *playwright.Playwright
	Browser playwright.Browser
	Domain  string
	Page    playwright.Page
	Entries []Entry
	File    *os.File
}

func NewScraper(config ScraperConfig) (*Scraper, error) {
	if config.Domain == "" {
		return nil, errors.New("domain is required")
	}

	if config.StartDate == "" {
		return nil, errors.New("start date is required")
	}

	if config.EndDate == "" {
		return nil, errors.New("end date is required")
	}

	// create  csv file
	f, err := os.Create("entries.csv")
	if err != nil {
		return nil, err
	}

	// write headers
	if _, err := f.WriteString("nomenclatura,entidad_convocante,pagina_web,telefono_entidad,objeto_contratacion,descripcion_objeto,valor_referencial,fecha_publicacion,etapa_1,etapa_1_fecha_inicio,etapa_1_fecha_termino,etapa_2,etapa_2_fecha_inicio,etapa_2_fecha_fin,ruc_entidad_contratante\n"); err != nil {
		return nil, err
	}

	if config.Debug {
		f, err := os.Create("log.txt")
		if err != nil {
			return nil, err
		}

		log.SetOutput(f)
	}

	pw, err := playwright.Run(&playwright.RunOptions{
		Verbose: true,
	})
	if err != nil {
		return nil, err
	}

	launchOptions := &playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(config.Headless),
	}

	browser, err := pw.Chromium.Launch(*launchOptions)
	if err != nil {
		return nil, err
	}

	page, err := browser.NewPage()
	if err != nil {
		return nil, err

	}

	if _, err = page.Goto(config.Domain); err != nil {
		return nil, err
	}

	return &Scraper{Pw: pw, Browser: browser, Page: page, Domain: config.Domain, File: f}, nil
}

func (s *Scraper) Search() error {
	// "Busqueda avanzada"
	if err := s.Page.Locator("legend.ui-fieldset-legend.ui-corner-all.ui-state-default").Click(); err != nil {
		return errors.New("error clicking advanced search button")
	}

	// "Fecha de inicio"
	startDate := s.Page.Locator("#tbBuscador\\:idFormBuscarProceso\\:dfechaInicio_input")
	if startDate == nil {
		return errors.New("error getting start date input")
	}

	if err := startDate.Fill("01/01/2024"); err != nil {
		return errors.New("error filling start date input")
	}

	// "Fecha de fin"
	endDate := s.Page.Locator("#tbBuscador\\:idFormBuscarProceso\\:dfechaFin_input")
	if endDate == nil {
		return errors.New("error getting end date input")
	}
	if err := endDate.Fill("31/10/2024"); err != nil {
		return errors.New("error filling end date input")
	}

	// "Buscar"
	if err := s.Page.Locator("#tbBuscador\\:idFormBuscarProceso\\:btnBuscarSelToken").Click(); err != nil {
		return errors.New("error clicking search button")
	}

	time.Sleep(3 * time.Second)

	return nil
}

func (s *Scraper) GetEntries() error {

	totalSearchPages, err := s.getNumberOfSearchPages()
	if err != nil {
		return err
	}

	for currentPage := 1; currentPage <= totalSearchPages; currentPage++ {

		table := s.Page.Locator("#tbBuscador\\:idFormBuscarProceso\\:dtProcesos_data")
		if table == nil {
			return errors.New("error getting table")
		}

		// Get Entries
		entryRows, err := table.Locator("tr").All()
		if err != nil {
			return errors.New("error getting trs")
		}

		time.Sleep(1 * time.Second)

		// Iterate over each Entry
		for idx, entryRow := range entryRows {

			// Get all columns for ewach entry
			entryColumns, err := entryRow.Locator("td").All()
			if err != nil {
				return errors.New("error getting tds")
			}

			// wait to load
			time.Sleep(5 * time.Second)

			nombre, err := entryColumns[1].InnerText()
			if err != nil {
				return errors.New("error getting nombre")
			}

			fechaPublicacion, err := entryColumns[2].InnerText()
			if err != nil {
				return errors.New("error getting fecha de publicacion")
			}

			nomenclatura, err := entryColumns[3].InnerText()
			if err != nil {
				return errors.New("error getting nomenclatura")
			}

			objetoDeContratacion, err := entryColumns[5].InnerText()
			if err != nil {
				return errors.New("error getting objeto de contratacion")
			}

			description, err := entryColumns[6].InnerText()
			if err != nil {
				return errors.New("error getting description")
			}

			value, err := entryColumns[9].InnerText()
			if err != nil {
				return errors.New("error getting value")
			}

			col12 := entryColumns[12]

			links, err := col12.Locator("a").All()
			if err != nil {
				return errors.New("error getting links")
			}

			// Ficha seleccion
			if err := links[1].Click(); err != nil {
				return errors.New("error clicking link to get entry details")
			}

			time.Sleep(3 * time.Second)

			// Datos generales (pagina web, telefono)
			paginaWeb, telefono, err := s.getWebsiteAndPhone()
			if err != nil {
				return err
			}

			// Cronograma
			convocatoria, fechaInicio, fechaFin, registroParticipantes, registroParticipantesInicio, registroParticipantesFin, err := s.getCronograma()
			if err != nil {
				return err
			}

			// get RUC
			ruc, err := s.getRUC()
			if err != nil {
				return err
			}

			// Write the entry to the CSV
			row := []string{
				cleanText(nomenclatura),
				cleanText(nombre),
				paginaWeb,
				telefono,
				cleanText(objetoDeContratacion),
				cleanText(description),
				cleanText(value),
				fechaPublicacion,
				convocatoria,
				fechaInicio,
				fechaFin,
				registroParticipantes,
				registroParticipantesInicio,
				registroParticipantesFin,
				ruc,
			}

			csvWriter := csv.NewWriter(s.File)
			if err := csvWriter.Write(row); err != nil {
				return err
			}

			csvWriter.Flush()

			// go back to previous page
			if err := s.previousPage(); err != nil {
				return err
			}

			if idx == 14 {
				s.nextPage(currentPage + 1)
				log.Println("Going to Next page: ", currentPage+1)
				break
			}
		}
	}

	return nil
}

func (s *Scraper) Close() error {
	if err := s.Browser.Close(); err != nil {
		return err
	}
	return s.Pw.Stop()
}

func (s *Scraper) getWebsiteAndPhone() (paginaWeb, telefono string, err error) {
	// Obtener tabla general table id="tbFicha:j_idt23"
	tableGeneral := s.Page.Locator("#tbFicha\\:j_idt23")
	if tableGeneral == nil {
		return "", "", errors.New("error getting table general")
	}

	// Obtener Tabla "Informacion General de la Entidad" table id=tbFicha:j_idt68
	informacionGeneralTable := tableGeneral.Locator("#tbFicha\\:j_idt68")
	if informacionGeneralTable == nil {
		return "", "", errors.New("error getting informacion general table")
	}

	// Obtener campos de la tabla "Informacion General de la Entidad"
	informacionGeneralRows, err := informacionGeneralTable.Locator("tr").All()
	if err != nil {
		return "", "", errors.New("error getting informacion general columns")
	}

	for i, row := range informacionGeneralRows {
		// obtain each field ionside row td
		switch i {
		case 2:
			columns, err := row.Locator("td").All()
			if err != nil {
				return "", "", errors.New("error getting columns")
			}

			paginaWeb, err = columns[1].InnerText()
			if err != nil {
				return "", "", errors.New("error getting pagina web")
			}

		case 3:
			columns, err := row.Locator("td").All()
			if err != nil {
				return "", "", errors.New("error getting columns")
			}

			telefono, err = columns[1].InnerText()
			if err != nil {
				return "", "", errors.New("error getting telefono entidad")
			}
		}
	}

	return cleanText(paginaWeb), cleanText(telefono), nil
}

func (s *Scraper) getNumberOfSearchPages() (int, error) {
	var totalSearchPages int

	// get total pages span class=ui-paginator-current
	totalPagesContainers, err := s.Page.Locator("span.ui-paginator-current").All()
	if err != nil {
		return 0, errors.New("error getting total pages container" + err.Error())
	}

	for _, container := range totalPagesContainers {
		totalPages, err := container.InnerText()
		if err != nil {
			return 0, errors.New("error getting total pages" + err.Error())
		}

		if strings.Contains(strings.ToLower(totalPages), strings.ToLower("[ Mostrando de 1 a 15")) {
			totalPages = strings.Split(totalPages, ":")[1]
			totalPages = strings.Split(totalPages, "/")[1]
			totalPages = strings.Trim(totalPages, "]")
			totalPages = strings.Trim(totalPages, " ")
			totalSearchPages, err = strconv.Atoi(totalPages)
			if err != nil {
				return 0, errors.New("error converting total pages to int: " + err.Error())
			}

			return totalSearchPages, nil
		}
	}

	return 0, errors.New("error getting total pages")
}

func (s *Scraper) getCronograma() (convocatoria, fechaInicio, fechaFin, registroParticipantes, registroParticipantesInicio, registroParticipantesFin string, err error) {
	// Locate the general cronograma table
	cronogrameTableGeneral := s.Page.Locator("#tbFicha\\:pnlContenedorFicha2")
	if cronogrameTableGeneral == nil {
		return "", "", "", "", "", "", errors.New("error getting cronograma table")
	}

	// Locate the specific cronograma table body
	cronogramaTable := cronogrameTableGeneral.Locator("#tbFicha\\:dtCronograma_data")
	if cronogramaTable == nil {
		return "", "", "", "", "", "", errors.New("error getting cronograma table body")
	}

	// Get all rows from the cronograma table
	cronogramaRows, err := cronogramaTable.Locator("tr").All()
	if err != nil {
		return "", "", "", "", "", "", errors.New("error getting cronograma rows")
	}

	// Iterate over the rows and extract the relevant information
	for idx, row := range cronogramaRows {
		// Get all columns for the current row
		columns, err := row.Locator("td").All()
		if err != nil {
			return "", "", "", "", "", "", errors.New("error getting columns")
		}

		// Process the first row (general cronograma details)
		if idx == 0 {
			convocatoria, err = columns[0].InnerText()
			if err != nil {
				return "", "", "", "", "", "", errors.New("error getting etapa for row 0")
			}

			if !strings.Contains(convocatoria, "Convocatoria") {
				convocatoria = "No especificado"
				fechaInicio = "No especificado"
				fechaFin = "No especificado"
				continue
			}

			fechaInicio, err = columns[1].InnerText()
			if err != nil {
				return "", "", "", "", "", "", errors.New("error getting fecha inicio for row 0")
			}

			fechaFin, err = columns[2].InnerText()
			if err != nil {
				return "", "", "", "", "", "", errors.New("error getting fecha fin for row 0")
			}

		} else if idx == 1 { // Process the second row (registration details)
			registroParticipantes, err = columns[0].InnerText()
			if err != nil {
				return "", "", "", "", "", "", errors.New("error getting etapa for row 1")
			}

			if !strings.Contains(registroParticipantes, "Registro de Participantes") && !strings.Contains(registroParticipantes, "Registro de participantes(Electronica)") {
				registroParticipantes = "No especificado"
				registroParticipantesInicio = "No especificado"
				registroParticipantesFin = "No especificado"
				continue
			}

			registroParticipantesInicio, err = columns[1].InnerText()
			if err != nil {
				return "", "", "", "", "", "", errors.New("error getting fecha inicio for row 1")
			}

			registroParticipantesFin, err = columns[2].InnerText()
			if err != nil {
				return "", "", "", "", "", "", errors.New("error getting fecha fin for row 1")
			}

		}
	}

	return cleanText(convocatoria), cleanText(fechaInicio), cleanText(fechaFin), cleanText(registroParticipantes), cleanText(registroParticipantesInicio), cleanText(registroParticipantesFin), nil
}

func (s *Scraper) getRUC() (ruc string, err error) {
	entidadContratanteTable := s.Page.Locator("#tbFicha\\:dtEntidadContrata_data")
	if entidadContratanteTable == nil {
		return "", errors.New("error getting entidad contratante table")
	}

	time.Sleep(3 * time.Second)

	// find first td role=gridcell
	entidadContratanteRows, err := entidadContratanteTable.Locator("td[role=gridcell]").All()
	if err != nil {
		return "", errors.New("error getting entidad contratante rows")
	}

	time.Sleep(2 * time.Second)

	for i, row := range entidadContratanteRows {
		if i == 0 {
			ruc, err = row.InnerText()
			if err != nil {
				return "", errors.New("error getting ruc")
			}
			break
		}
	}

	return cleanText(ruc), nil
}

func (s *Scraper) previousPage() error {
	if err := s.Page.Locator("#tbFicha\\:j_idt19").Click(); err != nil {
		return errors.New("error clicking back button")
	}

	time.Sleep(5 * time.Second)

	return nil
}

func (s *Scraper) nextPage(nextPage int) error {

	nextPageStr := strconv.Itoa(nextPage)

	// find the nav tbBuscador:idFormBuscarProceso:dtProcesos_paginator_bottom
	nav := s.Page.Locator("#tbBuscador\\:idFormBuscarProceso\\:dtProcesos_paginator_bottom")
	if nav == nil {
		return errors.New("error getting nav")
	}

	// get all spans ui-paginator-page ui-state-default ui-corner-all
	pages, err := nav.Locator("span.ui-paginator-page.ui-state-default.ui-corner-all").All()
	if err != nil {
		return errors.New("error getting pages")
	}

	// find the next page
	for _, page := range pages {
		text, err := page.InnerText()
		if err != nil {
			return errors.New("error getting page text")
		}

		if text == nextPageStr {
			if err := page.Click(); err != nil {
				return errors.New("error clicking next page")
			}

			time.Sleep(3 * time.Second)
			break
		}
	}

	return nil
}

func cleanText(text string) string {
	// remove extra spaces
	text = strings.TrimSpace(text)

	// remove new lines
	text = strings.ReplaceAll(text, "\n", "")

	// remove tabs
	text = strings.ReplaceAll(text, "\t", "")

	// remove commas, semicolons, quotes
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, ";", "")
	text = strings.ReplaceAll(text, "\"", "")

	// remove multiple spaces
	text = strings.Join(strings.Fields(text), " ")

	return text
}
