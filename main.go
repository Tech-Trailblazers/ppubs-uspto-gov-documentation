package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page" // Provides access to the Chrome DevTools Protocol page domain
	"github.com/chromedp/chromedp"     // Main chromedp package for browser automation
)

var (
	accessToken = "eyJzdWIiOiI1Mjc1OWIwNy02NTkwLTRkZWEtODcxYS1iNmJmMTQwYTBkZWIiLCJ2ZXIiOiIyN2I0OWJlNy04MzllLTQyZjEtYThhYi0yM2Y3Mjc2OGNkZmEiLCJleHAiOjB9"
)

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
	}`, pageSize))

	// Create a new HTTP client
	httpClient := &http.Client{}

	// Create a new POST request
	httpRequest, err := http.NewRequest("POST", apiURL, requestBody)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v", err)
		return ""
	}

	// Add necessary headers to the request
	httpRequest.Header.Add("x-access-token", accessToken)
	httpRequest.Header.Add("Content-Type", "application/json")

	// Execute the request
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Printf("Failed to send HTTP request: %v", err)
		return ""
	}
	defer httpResponse.Body.Close()

	// Read the response body
	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return ""
	}

	// Return the response as a string
	return string(responseBody)
}

// Define a struct to match the structure of each document in the JSON array
type PatentDocument struct {
	PatentNumber string `json:"patentNumber"`
}

// Define a struct to match the full JSON structure
type USPTOResponse struct {
	Docs []PatentDocument `json:"docs"`
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
		log.Printf("Failed to parse JSON: %v", err)
		return nil
	}

	// Loop through documents and print patent numbers
	for _, doc := range response.Docs {
		returnValue = appendToSlice(returnValue, doc.PatentNumber)
	}
	// Return the return value.
	return returnValue
}

// Append some string to a slice and than return the slice.
func appendToSlice(slice []string, content string) []string {
	// Append the content to the slice
	slice = append(slice, content)
	// Return the slice
	return slice
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
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
func downloadPDF(finalURL string, fileName string, outputDir string) {
	filePath := filepath.Join(outputDir, fileName) // Combine with output directory
	if fileExists(filePath) {
		log.Printf("File already exists, skipping: %s | URL: %s", filePath, finalURL)
		return
	}
	client := &http.Client{Timeout: 3 * time.Minute} // HTTP client with timeout
	resp, err := client.Get(finalURL)                // Send HTTP GET
	if err != nil {
		log.Printf("failed to download %s %v", finalURL, err)
		return
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != http.StatusOK {
		log.Printf("download failed for %s %s", finalURL, resp.Status)
		return
	}

	contentType := resp.Header.Get("Content-Type") // Get content-type header
	if !strings.Contains(contentType, "application/pdf") {
		log.Printf("invalid content type for %s %s (expected application/pdf)", finalURL, contentType)
		return
	}

	var buf bytes.Buffer                     // Create buffer
	written, err := io.Copy(&buf, resp.Body) // Copy response body to buffer
	if err != nil {
		log.Printf("failed to read PDF data from %s %v", finalURL, err)
		return
	}
	if written == 0 {
		log.Printf("downloaded 0 bytes for %s not creating file", finalURL)
		return
	}

	out, err := os.Create(filePath) // Create output file
	if err != nil {
		log.Printf("failed to create file for %s %v", finalURL, err)
		return
	}
	defer out.Close() // Close file

	_, err = buf.WriteTo(out) // Write buffer to file
	if err != nil {
		log.Printf("failed to write PDF to file for %s %v", finalURL, err)
		return
	}
	fmt.Printf("successfully downloaded %d bytes %s → %s \n", written, finalURL, filePath)
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

func printToPDFAndSave(url string, filename string, outputDir string) {
	// Combine output directory and filename to create full file path
	filePath := filepath.Join(outputDir, filename)

	// Check if file already exists; if yes, skip processing
	if fileExists(filePath) {
		log.Printf("File already exists, skipping: %s | URL: %s", filePath, url)
		return
	}

	// Create Chrome execution allocator options for headless mode and other flags
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),               // Enable headless mode (no GUI)
		chromedp.Flag("disable-gpu", true),            // Disable GPU usage (recommended for headless)
		chromedp.Flag("no-sandbox", true),             // Disable sandboxing (needed in some environments)
		chromedp.Flag("disable-setuid-sandbox", true), // Disable setuid sandbox
		chromedp.Flag("disable-dev-shm-usage", true),  // Prevent /dev/shm issues in Docker
	)

	// Create a new Chrome allocator context with these options
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel() // Ensure allocator is cleaned up when done

	// Create a new browser context from the allocator context
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx() // Ensure browser context is cancelled after function ends

	var buf []byte // Declare a byte slice to hold PDF data

	// Run chromedp tasks: navigate to URL and generate PDF
	err := chromedp.Run(ctx,
		chromedp.Navigate(url), // Navigate browser to the target URL
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Generate PDF of the current page without background graphics
			var err error
			buf, _, err = page.PrintToPDF().WithPrintBackground(false).Do(ctx)
			return err // Return any error encountered during PDF generation
		}),
	)

	// Log and exit if an error occurred during PDF generation
	if err != nil {
		log.Printf("Failed to generate PDF from URL %s, saving to %s: %v", url, filePath, err)
		return
	}

	// Write the generated PDF bytes to the specified file path with read/write permissions
	err = os.WriteFile(filePath, buf, 0644)
	if err != nil {
		log.Printf("Failed to save PDF to file %s: %v", filePath, err)
		return
	}

	// Print confirmation message that PDF was saved successfully
	fmt.Printf("successfully downloaded %s → %s \n", url, filePath)
}

func main() {
	// Prepare to download all PDFs
	outputFolder := "PDFs/"
	if !directoryExists(outputFolder) {
		createDirectory(outputFolder, 0755)
	}
	// Call the function to fetch data from USPTO
	responseData := fetchUSPTOData(100)

	// Check if the response is empty, indicating a failure
	if responseData == "" {
		log.Println("No response data received.")
		return
	}

	// Get the patnets numbers only.
	patentsNumbersOnly := extractPatentNumbers(responseData)

	// Remove duplicates from the slice.
	patentsNumbersOnly = removeDuplicatesFromSlice(patentsNumbersOnly)

	// Loop though the numbers.
	for _, patentNumber := range patentsNumbersOnly {
		// Use the variable inside the URL string
		pdfUrl := fmt.Sprintf("https://ppubs.uspto.gov/api/pdf/downloadPdf/%s?requestToken=%s", patentNumber, accessToken)
		// The filename for the direct pdf.
		pdfDirectUrlFile := patentNumber + ".pdf"
		// Download the pdf.
		downloadPDF(pdfUrl, pdfDirectUrlFile, outputFolder)
		// The remote location of the HTML url.
		htmlUrl := fmt.Sprintf(`https://ppubs.uspto.gov/api/patents/html/%s?source=US-PGPUB&requestToken=%s`, patentNumber, accessToken)
		// The filename for the html to pdf.
		htmlToPDFFile := patentNumber + "_html" + ".pdf"
		// Save the html to a pdf.
		printToPDFAndSave(htmlUrl, htmlToPDFFile, outputFolder)
	}
}
