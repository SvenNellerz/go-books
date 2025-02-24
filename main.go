package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

// Book struct to hold book data.
type Book struct {
	Title string `json:"title"`
}

// searchResults holds the API response structure.
type searchResults struct {
	Docs []Book `json:"docs"`
}

// searchHandler handles HTTP requests to search for books by author.
func searchHandler(w http.ResponseWriter, r *http.Request) {
	author := r.URL.Query().Get("author")
	if author == "" {
		http.Error(w, "Missing 'author' query parameter", http.StatusBadRequest)
		return
	}

	safeAuthor := url.QueryEscape(author)
	apiURL := fmt.Sprintf("https://openlibrary.org/search.json?author=%s", safeAuthor)
	resp, err := http.Get(apiURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching data: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var results searchResults
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding data: %v", err), http.StatusInternalServerError)
		return
	}

	if len(results.Docs) == 0 {
		http.Error(w, fmt.Sprintf("No books found for author %s", author), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/search", searchHandler)

	// Use the PORT environment variable if available, else default to 8080.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
