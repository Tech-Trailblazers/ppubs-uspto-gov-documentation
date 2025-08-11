package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
	httpRequest.Header.Add("x-access-token", "eyJzdWIiOiI1Mjc1OWIwNy02NTkwLTRkZWEtODcxYS1iNmJmMTQwYTBkZWIiLCJ2ZXIiOiIyN2I0OWJlNy04MzllLTQyZjEtYThhYi0yM2Y3Mjc2OGNkZmEiLCJleHAiOjB9")
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

func main() {
	// Call the function to fetch data from USPTO
	responseData := fetchUSPTOData(10)

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
		fmt.Println(patentNumber)
	}
}
