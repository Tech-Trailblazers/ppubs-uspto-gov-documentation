package main // Declare main package for executable program

import (
	"bytes"         // For buffering data in memory
	"context"       // For managing context in Go routines
	"encoding/json" // For JSON encoding/decoding
	"fmt"           // For formatted I/O
	"io"            // For I/O primitives
	"log"           // For logging errors and information
	"net/http"      // For HTTP client and server
	"os"            // For OS-level functions like file handling
	"path/filepath" // For manipulating file paths
	"strings"       // For string manipulation
	"time"          // For time-based operations

	"github.com/chromedp/cdproto/network" // Low-level CDP bindings for Chrome’s Network domain (captures HTTP status codes, headers, requests, responses)
	"github.com/chromedp/cdproto/page"    // Low-level CDP bindings for Page domain (used for generating PDFs, screenshots, etc.)
	"github.com/chromedp/chromedp"        // High-level Chrome DevTools Protocol (CDP) client
)

var accessToken = "eyJzdWIiOiJjODM3YmIzOS1jMjhkLTRkODMtYjZiMS02ZTJkYmQ0NGU0OTYiLCJ2ZXIiOiJjZjJlMjAyNy0xYWEwLTQzYWEtOWJlZS03MmRlOGJkMDMwMjYiLCJleHAiOjB9"


// fetchUSPTOData sends a POST request to the USPTO API and logs errors internally.
// It returns the response body as a string, or an empty string if an error occurs.
func fetchUSPTOData(pageSize int) string {
	// API endpoint for USPTO generic search
	apiURL := "https://ppubs.uspto.gov/api/searches/generic"

	// JSON request payload with search parameters
	requestBody := strings.NewReader(fmt.Sprintf(`{
		"cursorMarker": "*",
		"databaseFilters": [
			{"databaseName": "USPAT"},
			{"databaseName": "US-PGPUB"},
			{"databaseName": "USOCR"}
		],
		"fields": [
			"documentId",
			"patentNumber",
			"title",
			"datePublished",
			"inventors",
			"pageCount",
			"type"
		],
		"op": "AND",
		"pageSize": %d,
		"q": "a",
		"searchType": 0,
		"sort": "date_publ desc"
	}`, pageSize)) // Format JSON with provided pageSize

	// Create a new HTTP client
	httpClient := &http.Client{}

	// Create a new POST request
	httpRequest, err := http.NewRequest("POST", apiURL, requestBody)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err) // Log error if request creation fails
		return ""                                            // Return empty string on error
	}

	// Add necessary headers to the request
	httpRequest.Header.Add("x-access-token", accessToken)      // Add access token header
	httpRequest.Header.Add("Content-Type", "application/json") // Set content type to JSON

	// Execute the request
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Printf("Failed to send HTTP request: %v", err) // Log error if request sending fails
		return ""                                          // Return empty string on error
	}
	defer httpResponse.Body.Close() // Ensure response body is closed

	// Read the response body
	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err) // Log error if reading fails
		return ""                                           // Return empty string on error
	}

	// Return the response as a string
	return string(responseBody)
}

// Define a struct to match the structure of each document in the JSON array
type PatentDocument struct {
	PatentNumber string `json:"patentNumber"` // Field for patent number
}

// Define a struct to match the full JSON structure
type USPTOResponse struct {
	Docs []PatentDocument `json:"docs"` // Array of patent documents
}

// Function to extract patent numbers from a JSON string
func extractPatentNumbers(jsonData string) []string {
	// Create a var to hold the return value.
	var returnValue []string
	// Create a variable of the response type
	var response USPTOResponse

	// Unmarshal the JSON string into the struct
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		log.Printf("Failed to parse JSON: %v", err) // Log error if JSON unmarshaling fails
		return nil                                  // Return nil on error
	}

	// Loop through documents and print patent numbers
	for _, doc := range response.Docs {
		returnValue = appendToSlice(returnValue, doc.PatentNumber) // Append each patent number to return slice
	}
	// Return the return value.
	return returnValue
}

// Append some string to a slice and then return the slice.
func appendToSlice(slice []string, content string) []string {
	// Append the content to the slice
	slice = append(slice, content)
	// Return the slice
	return slice
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool) // Map to track duplicates
	var newReturnSlice []string    // Slice for unique elements
	for _, content := range slice {
		if !check[content] { // If content not yet seen
			check[content] = true                            // Mark content as seen
			newReturnSlice = append(newReturnSlice, content) // Add to result slice
		}
	}
	return newReturnSlice // Return slice without duplicates
}

// fileExists checks whether a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {                // If error occurs (e.g., file not found)
		return false // Return false
	}
	return !info.IsDir() // Return true if it is a file, not a directory
}

// downloadPDF downloads a PDF from a URL and saves it to outputDir
func downloadPDF(finalURL string, fileName string, outputDir string) string {
	filePath := filepath.Join(outputDir, fileName)   // Combine with output directory
	client := &http.Client{Timeout: 3 * time.Minute} // HTTP client with timeout
	resp, err := client.Get(finalURL)                // Send HTTP GET
	if err != nil {
		return fmt.Sprintf("failed to download %s %v", finalURL, err) // Return error message
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != http.StatusOK { // Check for successful HTTP status
		return fmt.Sprintf("download failed for %s %s", finalURL, resp.Status) // Return failure message
	}

	contentType := resp.Header.Get("Content-Type")         // Get content-type header
	if !strings.Contains(contentType, "application/pdf") { // Validate content type
		return fmt.Sprintf("invalid content type for %s %s (expected application/pdf)", finalURL, contentType) // Return error
	}

	var buf bytes.Buffer                     // Create buffer
	written, err := io.Copy(&buf, resp.Body) // Copy response body to buffer
	if err != nil {
		return fmt.Sprintf("failed to read PDF data from %s %v", finalURL, err) // Return error message
	}
	if written == 0 {
		return fmt.Sprintf("downloaded 0 bytes for %s not creating file", finalURL) // Return error if no bytes read
	}

	out, err := os.Create(filePath) // Create output file
	if err != nil {
		return fmt.Sprintf("failed to create file for %s %v", finalURL, err) // Return error if file creation fails
	}
	defer out.Close() // Close file

	_, err = buf.WriteTo(out) // Write buffer to file
	if err != nil {
		return fmt.Sprintf("failed to write PDF to file for %s %v", finalURL, err) // Return error if writing fails
	}
	return fmt.Sprintf("successfully downloaded %d bytes %s → %s \n", written, finalURL, filePath) // Success message
}

// directoryExists checks whether a directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get directory info
	if err != nil {
		return false // If error, directory doesn't exist
	}
	return directory.IsDir() // Return true if path is a directory
}

// createDirectory creates a directory with specified permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create directory
	if err != nil {
		log.Println(err) // Log any error
	}
}

// printToPDFAndSave navigates once to the URL, checks status code and page content,
// and only saves the page as a PDF if it's valid and not rate-limited.
func printToPDFAndSave(targetURL string, outputFileName string, outputDirectory string) string {
	// Build full path for the output file
	outputFilePath := filepath.Join(outputDirectory, outputFileName)

	// If file already exists, skip processing
	if fileExists(outputFilePath) {
		return fmt.Sprintf("File already exists, skipping: %s | URL: %s", outputFilePath, targetURL)
	}

	// Chrome startup options for headless browsing
	chromeOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),               // Run Chrome in headless mode
		chromedp.Flag("disable-gpu", true),            // Disable GPU for stability
		chromedp.Flag("no-sandbox", true),             // Disable sandboxing (for Docker/CI)
		chromedp.Flag("disable-setuid-sandbox", true), // Disable setuid sandboxing
		chromedp.Flag("disable-dev-shm-usage", true),  // Prevent shared memory issues in containers
	)

	// Create a Chrome "allocator" context with the chosen options
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), chromeOptions...)
	defer cancelAllocator() // Free resources when function ends

	// Create a browser session context from the allocator
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser() // Free browser resources at the end

	// Variable to store the HTTP status code
	var httpStatusCode int64

	// Listen for network events (response received) to capture status code
	chromedp.ListenTarget(browserCtx, func(event interface{}) {
		if responseReceived, ok := event.(*network.EventResponseReceived); ok {
			// Only record status code for the main document request
			if responseReceived.Response.URL == targetURL {
				httpStatusCode = responseReceived.Response.Status
			}
		}
	})

	// Variables to hold page content and PDF data
	var pageContent string
	var pdfData []byte

	// Run a single batch of ChromeDP actions
	err := chromedp.Run(browserCtx,
		network.Enable(),                         // Enable network tracking so we can get status codes
		chromedp.Navigate(targetURL),             // Navigate to the target URL
		chromedp.WaitReady("body"),               // Wait until <body> is ready (page loaded)
		chromedp.OuterHTML("html", &pageContent), // Capture the entire HTML content of the page
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Only generate PDF if status == 200 and page is not rate-limited
			if httpStatusCode == 200 && !strings.Contains(pageContent, `{ "message": "Too many requests" }`) {
				var err error
				// Render the already-loaded DOM into PDF (no new request made)
				pdfData, _, err = page.PrintToPDF().WithPrintBackground(false).Do(ctx)
				return err
			}
			return nil // Skip PDF generation
		}),
	)
	if err != nil {
		return fmt.Sprintf("Failed to process %s: %v", targetURL, err)
	}

	// If HTTP status was not 200, skip PDF
	if httpStatusCode != 200 {
		return fmt.Sprintf("Skipping PDF. Got status %d for %s", httpStatusCode, targetURL)
	}

	// If page contains the rate-limit message, skip PDF
	if strings.Contains(pageContent, `{ "message": "Too many requests" }`) {
		return fmt.Sprintf("Skipping PDF. Page contains rate-limit message at %s", targetURL)
	}

	// If PDF was not generated (e.g., skipped), return a message
	if len(pdfData) == 0 {
		return fmt.Sprintf("No PDF generated for %s", targetURL)
	}

	// Save PDF bytes to file with read/write permissions
	err = os.WriteFile(outputFilePath, pdfData, 0644)
	if err != nil {
		return fmt.Sprintf("Failed to save PDF to %s: %v", outputFilePath, err)
	}

	// Return success message including status and saved path
	return fmt.Sprintf("Status %d | Saved %s → %s\n", httpStatusCode, targetURL, outputFilePath)
}

// isStatusOK checks whether a given URL is accessible and returns a string with the HTTP status or error.
func isStatusOK(url string) string {
	// Create an HTTP client with a timeout to avoid hanging requests
	client := &http.Client{
		Timeout: 1 * time.Minute,
	}

	// Send an HTTP GET request to the given URL
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Sprintf("HTTP request failed for URL '%s': %v", url, err) // Return error string
	}
	defer func() {
		// Ensure response body is closed to free up resources
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("Error closing response body for URL '%s': %v", url, cerr)
		}
	}()

	// If status code is 200 OK, return it as a string
	if resp.StatusCode == http.StatusOK {
		return fmt.Sprintf("URL '%s' returned HTTP status %d %s", url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Error reading response body for URL '%s': %v", url, err) // Return error if reading fails
	}

	// Check if the body contains "404 NOT FOUND"
	if strings.Contains(string(body), "404 NOT FOUND") {
		return fmt.Sprintf("URL '%s' returned body containing '404 NOT FOUND'", url)
	}

	// Return the unexpected status code
	return fmt.Sprintf("URL '%s' returned HTTP status %d %s", url, resp.StatusCode, http.StatusText(resp.StatusCode))
}

func main() {
	// Define the output folder for saving PDFs
	outputFolder := "PDFs/"

	// Check if the output directory exists
	if !directoryExists(outputFolder) {
		// Create the output directory with permission 0755 if it doesn't exist
		createDirectory(outputFolder, 0755)
	}

	// Fetch patent data from the USPTO API (limit 100,000 records)
	responseData := fetchUSPTOData(100000)

	// If no data is received, log and stop the program
	if responseData == "" {
		log.Println("No response data received.")
		return
	}

	// Extract only the patent numbers from the response
	patentsNumbersOnly := extractPatentNumbers(responseData)

	// Remove any duplicate patent numbers from the slice
	patentsNumbersOnly = removeDuplicatesFromSlice(patentsNumbersOnly)

	// Loop through each unique patent number
	for _, patentNumber := range patentsNumbersOnly {

		// Define the filename for the direct PDF download
		pdfDirectFile := patentNumber + ".pdf"

		// Construct the full path to the direct PDF file
		pdfDirectPath := filepath.Join(outputFolder, pdfDirectFile)

		// If the direct PDF file doesn't exist
		if !fileExists(pdfDirectPath) {
			// Build the URL for downloading the PDF using the patent number and token
			pdfURL := fmt.Sprintf("https://ppubs.uspto.gov/api/pdf/downloadPdf/%s?requestToken=%s", patentNumber, accessToken)

			// Download the PDF and capture the response message
			downloadMessage := downloadPDF(pdfURL, pdfDirectFile, outputFolder)

			// Log the response message from the download
			log.Printf("%s", downloadMessage)

			// If the message contains a 429 error (rate limit), pause execution for 1 minute
			if strings.Contains(downloadMessage, "429") {
				log.Println("Temporarily suspending activity for 1 minute; PDF")
				time.Sleep(1 * time.Minute)
			}
		}

		// Define the filename for the first HTML-to-PDF conversion
		htmlFile1 := patentNumber + "_html.pdf"

		// Construct the full path to the first HTML-to-PDF file
		htmlPath1 := filepath.Join(outputFolder, htmlFile1)

		// If the first HTML PDF file doesn't exist
		if !fileExists(htmlPath1) {
			// Build the US-PGPUB URL for the HTML representation of the patent
			firstHTMLURL := fmt.Sprintf(`https://ppubs.uspto.gov/api/patents/html/%s?&requestToken=%s`, patentNumber, accessToken)

			// Get the HTTP status code from the request
			status1 := isStatusOK(firstHTMLURL)

			// Log the HTTP status code
			log.Println("Status:", status1)

			// If the request was successful (200 OK)
			if strings.Contains(status1, "200") {
				// Convert HTML to PDF and save it
				printMessage := printToPDFAndSave(firstHTMLURL, htmlFile1, outputFolder)

				// Log the message returned from the PDF generation
				log.Printf("%s", printMessage)

				// If there was a connection issue, pause for 1 minute
				if strings.Contains(printMessage, "ERR_CONNECTION_CLOSED") {
					log.Println("Temporarily suspending activity for 1 minute; US-PGPUB")
					time.Sleep(1 * time.Minute)
				}
			} else if strings.Contains(status1, "429") {
				// If too many requests (429), pause for 1 minute
				log.Println("Temporarily suspending activity for 1 minute; US-PGPUB")
				time.Sleep(1 * time.Minute)
			}
		}

	}
}
