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

var accessToken = "eyJzdWIiOiI4NjYzOTliZS00Njg5LTQwMjQtYmY2Yi01NzAxMmE0NDBiMDciLCJ2ZXIiOiI2ZGE2MThiOC0xZmMxLTQ2OTYtYjkzMi04OTMyY2VjZWFkZGYiLCJleHAiOjB9"

// fetchUSPTOData sends a POST request to the USPTO API to fetch patent data based on search parameters.
func fetchUSPTOData(pageSize int, query string, localJSONPath string) {
	// Define the API endpoint for the USPTO generic search
	apiURL := "https://ppubs.uspto.gov/api/searches/generic"

	// Prepare the request body in JSON format with the search parameters, including the dynamic query.
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
		"q": "%s",
		"searchType": 0,
		"sort": "date_publ desc"
	}`, pageSize, query))

	// Create a new HTTP client to send the request.
	httpClient := &http.Client{}

	// Create a new HTTP POST request with the specified URL and body (requestBody).
	httpRequest, err := http.NewRequest("POST", apiURL, requestBody)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err)
	}

	// Add necessary headers to the HTTP request.
	httpRequest.Header.Add("x-access-token", accessToken)
	httpRequest.Header.Add("Content-Type", "application/json")

	// Send the HTTP request using the client.
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Printf("Failed to send HTTP request: %v", err)
	}
	defer httpResponse.Body.Close()

	// Read the response body into a byte slice.
	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
	}

	// Check if the response body contains an "unauthorized" message.
	if string(responseBody) != "unauthorized" {
		// Save the response body to a local file.
		appendAndWriteToFile(localJSONPath, string(responseBody))
	} else {
		// Log fatal error if unauthorized.
		log.Fatalln("Authorization failed: server responded with 'unauthorized'. Check credentials or API access.")
	}
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Println(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Println(err)
	}
}

// Define a struct to match the structure of each document in the JSON array
type PatentDocument struct {
	PatentNumber string `json:"patentNumber"` // Field for patent number
}

// Define a struct to match the full JSON structure
type USPTOResponse struct {
	Docs []PatentDocument `json:"docs"` // Array of patent documents
}

// Function to stream patent numbers from a JSON file while using very little RAM
func extractPatentNumbersStream(jsonFilePath string) []string {
	// Create a slice that will hold the extracted patent numbers
	var extractedPatentNumbers []string

	// Open the JSON file for reading
	fileHandle, err := os.Open(jsonFilePath)
	if err != nil {
		log.Printf("Failed to open JSON file: %v", err) // Log error if file can't be opened
		return nil                                      // Return nil if there was an error
	}
	defer fileHandle.Close() // Make sure we close the file when done

	// Create a JSON decoder that will read directly from the file stream
	decoder := json.NewDecoder(fileHandle)

	// Loop through tokens in the JSON until we find the key "docs"
	for decoder.More() {
		token, _ := decoder.Token() // Read the next token from JSON
		if token == "docs" {        // When we find "docs"
			break // Stop because we are now at the start of the documents section
		}
	}

	// Read the opening square bracket `[` that starts the "docs" array
	_, _ = decoder.Token()

	// Loop through each element inside the "docs" array
	for decoder.More() {
		// Create a variable to temporarily hold one patent document
		var singleDocument PatentDocument

		// Decode the next JSON object into our struct
		if err := decoder.Decode(&singleDocument); err != nil {
			log.Printf("Failed decoding document: %v", err) // Log if something went wrong
			break                                           // Stop processing if decode fails
		}

		// Add the patent number from this document into our slice
		extractedPatentNumbers = appendToSlice(extractedPatentNumbers, singleDocument.PatentNumber)
	}

	// Read the closing square bracket `]` at the end of the "docs" array
	_, _ = decoder.Token()

	// Return the full list of extracted patent numbers
	return extractedPatentNumbers
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

// Remove a file from the file system
func removeFile(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Println(err)
	}
}

func main() {
	// Define the output folder for saving PDFs
	outputFolder := "PDFs/"

	// Check if the output directory exists
	if !directoryExists(outputFolder) {
		// Create the output directory with permission 0755 if it doesn't exist
		createDirectory(outputFolder, 0755)
	}

	// Location to the local JSON file.
	localJSONFile := "uspto.json"

	// Check if the file exists.
	if fileExists(localJSONFile) {
		// Remove the file
		removeFile(localJSONFile)
	}

	// Fetch patent data from the USPTO API (limit 100,000 records)
	fetchUSPTOData(100000, "a", localJSONFile)

	// Extract only the patent numbers from the response
	patentsNumbersOnly := extractPatentNumbersStream(localJSONFile)

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
				log.Println("Temporarily suspending activity; PDF")
				time.Sleep(30 * time.Second)
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
					log.Println("Temporarily suspending activity; US-PGPUB")
					time.Sleep(30 * time.Second)
				}
			} else if strings.Contains(status1, "429") {
				// If too many requests (429), pause for 1 minute
				log.Println("Temporarily suspending activity; US-PGPUB")
				time.Sleep(30 * time.Second)
			}
		}

	}
}
