package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/html"

	"awesomeProject/models"
)

var ErrNotFound = errors.New("not found")

const (
	etfFinerWithFilter = "/us/en/individual/etfs/fund-finder?g=assetclass%3Aequity&tab=overview"

	// common selectors
	tableSelector     = "table.data-table"
	labelCellSelector = "td.label"
	dataCellSelector  = "td.data"

	// etf name selectors
	tickerSelector = "span.ticker"

	// etf description selectors
	descriptionSelector = "section.comp-text:has(h2.comp-title:contains('About this Benchmark')) div.ssmp-richtext"

	// etf top holdings selectors
	topHoldingsSectionSelector = "section:has(h3:contains('Top Holdings'))"

	// etf sector selectors
	sectorFundBreakdownDivSelector = "div[data-fundComponent='true']:has(h3:contains('Fund Sector Breakdown'))"
	sectorBreakdownDivSelector     = "div[data-fundComponent='true']:has(h3:contains('Sector Breakdown'))"
	sectorIndustryDivSelector      = "div[data-fundComponent='true']:has(h3:contains('Fund Industry Allocation'))"
	sectorSubIndustryDivSelector   = "div[data-fundComponent='true']:has(h3:contains('Fund Sub-Industry Allocation'))"

	// etf geographical selectors
	geographicalSelector = "input#fund-geographical-breakdown"

	semaphoreCapacity = 50
)

type DailyDataUpdater struct {
	host   string
	logger *logrus.Logger
	store  *Database
}

func NewDailyDataUpdater(host string, db *Database, log *logrus.Logger) *DailyDataUpdater {
	return &DailyDataUpdater{
		host:   host,
		logger: log,
		store:  db,
	}
}

func (u *DailyDataUpdater) Run() {
	// Attempt the initial update immediately
	err := u.UpdateData()
	if err != nil {
		u.logger.Fatalf("Initial data update failed: %v", err)
	}
	u.logger.Info("Data update completed successfully")

	// Create a ticker for daily updates
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Define the number of retries
	retries := 3

	for {
		select {
		case <-ticker.C:
			// Attempt the update with retries
			for i := 0; i < retries; i++ {
				err := u.UpdateData()
				if err == nil {
					u.logger.Info("Data update completed successfully")
					break
				} else {
					u.logger.Errorf("Data update attempt %d failed: %v", i+1, err)
					if i < retries-1 {
						u.logger.Infof("Retrying data update in 1 hour...")
						time.Sleep(1 * time.Hour)
					}
				}
			}
		}
	}
}

func (u *DailyDataUpdater) UpdateData() error {
	// Get data paths for updating
	fetchPaths, err := u.getPaths()
	if err != nil {
		return fmt.Errorf("failed to get data paths: %v", err)
	}

	// Use WaitGroup to wait for completion of all updates
	var wg sync.WaitGroup

	// Define a channel-based semaphore with a capacity
	semaphore := make(chan struct{}, semaphoreCapacity)

	// Launch data updates for each path in parallel
	for path := range fetchPaths {
		wg.Add(1)

		// Acquire a semaphore slot, limiting concurrent updates to 50
		semaphore <- struct{}{}

		go func(path string) {
			defer func() {
				// Release the semaphore slot when done
				<-semaphore
				wg.Done()
			}()

			u.updateETF(path)
		}(path)
	}

	// Wait for completion of all updates
	wg.Wait()

	return nil
}

func (u *DailyDataUpdater) updateETF(path string) {
	// Create the URL by combining the host and path
	url := u.host + path

	// Send an HTTP GET request to the URL
	resp, err := http.Get(url)
	if err != nil {
		u.logger.Error(err)
		return
	}
	defer resp.Body.Close()

	// Read the HTML content from the response body
	htmlContent, err := io.ReadAll(resp.Body)
	if err != nil {
		u.logger.Error(err)
		return
	}

	// Create a string reader from the HTML content
	reader := strings.NewReader(string(htmlContent))

	// Parse the HTML document using goquery
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		u.logger.Error(err)
		return
	}

	// Build an ETF object from the parsed HTML document
	etf, err := u.buildETF(doc)
	if err != nil {
		u.logger.Errorf("Could not build ETF, error: %s, URL: %s", err, url)
		return
	}

	// Upsert the ETF data into the database
	err = u.store.Upsert(etf)
	if err != nil {
		u.logger.Errorf("Could not upsert ETF, error: %s, URL: %s", err, url)
	}
}

func (u *DailyDataUpdater) buildETF(doc *goquery.Document) (etf models.ETF, err error) {
	var etfData models.ETFData

	// Extract and clean the ETF name from the HTML document
	etfData.Name = strings.TrimSpace(doc.Find(tickerSelector).Text())
	if etfData.Name == "" {
		return etf, errors.New("not found name")
	}

	// Extract the ETF description from the HTML document
	etfData.Description = doc.Find(descriptionSelector).Text()
	if etfData.Description == "" {
		return etf, errors.New("description found name")
	}

	// Find and populate the ETF's top holdings
	etfData.TopHoldings, err = u.findHoldings(doc)
	if err != nil {
		return etf, fmt.Errorf("findHoldings returns: %s", err)
	}

	// Find and populate the ETF's sectors data
	etfData.Sectors, err = u.findSectors(doc)
	if err != nil {
		return etf, fmt.Errorf("findSectors returns: %s", err)
	}

	// Find and populate the ETF's countries data
	etfData.Countries, err = u.findCountries(doc)
	if err != nil && err != ErrNotFound { // It's okay if ETF doesn't have geoData
		return etf, fmt.Errorf("findCountries returns: %s", err)
	}

	// Set the ETF's ID as its name and serialize the ETFData to JSON
	etf.ID = etfData.Name
	etf.Data = etfData.ToJson()

	return etf, nil
}

func (u *DailyDataUpdater) findHoldings(doc *goquery.Document) ([]models.Holding, error) {
	// Find the section containing the top holdings information with an <h3> element containing 'Top Holdings'.
	div := doc.Find(topHoldingsSectionSelector)

	// Check if the section exists
	if div.Length() == 0 {
		return nil, ErrNotFound
	}

	// Create a slice to store FundHoldings
	var fundHoldings []models.Holding

	// Iterate over the rows of the table, starting from the second row (skipping the header)
	div.Find(tableSelector).Find("tr").Each(func(index int, rowHtml *goquery.Selection) {
		if index > 0 && rowHtml.Find(labelCellSelector).Text() != "" {
			// Extract data from the cells in the row
			// we select Fund Top Holdings it's mean rowHtml.Find(dataCellSelector).Eq(1).Text() shouldn't be empty
			// rowHtml.Find(dataCellSelector).Eq(1).Text() empty for Index Top Holdings
			if rowHtml.Find(dataCellSelector).Eq(1).Text() != "" {
				holdingName := rowHtml.Find(labelCellSelector).Text()
				sharesHeld := rowHtml.Find(dataCellSelector).Eq(0).Text()
				weight := rowHtml.Find(dataCellSelector).Eq(1).Text()

				// Create a FundHoldings object and append it to the slice
				holding := models.Holding{
					Name:       holdingName,
					SharesHeld: sharesHeld,
					Weight:     weight,
				}
				fundHoldings = append(fundHoldings, holding)
			}
		}
	})

	return fundHoldings, nil
}

func (u *DailyDataUpdater) findSectors(doc *goquery.Document) ([]models.WeightData, error) {
	sectorDiv := &goquery.Selection{}
	sectorDivs := []string{
		sectorBreakdownDivSelector,
		sectorIndustryDivSelector,
		sectorSubIndustryDivSelector,
		sectorFundBreakdownDivSelector,
	}

	for i := range sectorDivs {
		sectorDiv = doc.Find(sectorDivs[i])
		if sectorDiv.Length() != 0 {
			break
		}
	}

	// Check if the div exists
	if sectorDiv.Length() == 0 {
		return nil, ErrNotFound
	}

	sectors := []models.WeightData{}

	// Iterate over the rows of the table, starting from the second row (skipping the header)
	sectorDiv.Find(tableSelector).Find("tr").Each(func(index int, rowHtml *goquery.Selection) {
		if index > 0 && rowHtml.Find(labelCellSelector).Text() != "" {
			// Extract data from the cells in the row
			name := rowHtml.Find(labelCellSelector).Text()
			weight := rowHtml.Find(dataCellSelector).Eq(0).Text()

			// Create a SectorWeight object and append it to the slice
			sector := models.WeightData{
				Name:   name,
				Weight: weight,
			}

			sectors = append(sectors, sector)
		}
	})

	return sectors, nil
}

func (u *DailyDataUpdater) findCountries(doc *goquery.Document) ([]models.WeightData, error) {
	// Find the input element with the specified ID
	inputElement := doc.Find(geographicalSelector)

	// Check if the inputElement exists
	if inputElement.Length() == 0 {
		return nil, ErrNotFound
	}

	// Get the value of the "value" attribute of this element
	value := inputElement.AttrOr("value", "")

	// Create a struct to unmarshal the JSON data
	var geoData models.GeographicalData

	// Unmarshal the JSON data into the struct
	err := json.Unmarshal([]byte(value), &geoData)
	if err != nil {
		return nil, err
	}

	return u.processGeographicalData(geoData), nil
}

func (u *DailyDataUpdater) processGeographicalData(geoData models.GeographicalData) []models.WeightData {
	result := make([]models.WeightData, len(geoData.AttributeArray))

	for i := range geoData.AttributeArray {
		result[i] = models.WeightData{
			Name:   geoData.AttributeArray[i].Name.Value,
			Weight: geoData.AttributeArray[i].Weight.Value,
		}
	}

	return result
}

// getPaths fetches URLs from a specific website using Playwright and parses the HTML.
func (u *DailyDataUpdater) getPaths() (map[string]struct{}, error) {
	// Launch Playwright and Chromium
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("could not launch playwright, err: %v", err)
	}

	browser, err := pw.Chromium.Launch()
	if err != nil {
		return nil, fmt.Errorf("could not launch Chromium, err: %v", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("could not create page, err: %v", err)
	}

	// Navigate to the target URL
	if _, err := page.Goto(u.host+etfFinerWithFilter, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return nil, fmt.Errorf("could not navigate to the URL, err: %v", err)
	}

	// Wait for the page to load completely
	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateLoad,
	})
	if err != nil {
		return nil, fmt.Errorf("could not wait for load state, err: %v", err)
	}

	// Get the inner HTML content of the ".tab-content" element
	htmlContent, err := page.Locator(".tab-content").InnerHTML()
	if err != nil {
		return nil, fmt.Errorf("could not get HTML content, err: %v", err)
	}

	// Close the browser and stop Playwright
	if err = browser.Close(); err != nil {
		u.logger.Errorf("Could not close the browser: %v", err)
	}
	if err = pw.Stop(); err != nil {
		u.logger.Errorf("Could not stop Playwright: %v", err)
	}

	// Create an HTML parser from the string
	reader := strings.NewReader(htmlContent)
	tokenizer := html.NewTokenizer(reader)

	// Function to clean URLs
	cleanURL := func(url string) string {
		index := strings.Index(url, "#")
		if index != -1 {
			return url[:index]
		}
		return url
	}

	// Parsing HTML and processing tags
	urls := map[string]struct{}{}

	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			return urls, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != "a" {
				continue
			}
			for _, attr := range token.Attr {
				if attr.Key == "href" {
					nu := cleanURL(attr.Val)
					urls[nu] = struct{}{}
				}
			}
		}
	}
}
