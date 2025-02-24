package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

// book struct to hold all book data
type Book struct {
	Title string `json:"title"`
}

type searchResults struct {
	Docs []Book `json:"docs"`
}

// Root Command
var rootcmd = &cobra.Command{
	Use:   "books",
	Short: "Books CLI is an application to fetch books by authors",
}

// Search Command
var cmsSearch = &cobra.Command{
	Use:   "search [author]",
	Short: "Search for books by a specific author",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		searchBooks(args[0])
	},
}

// function to search books by author via api call
func searchBooks(author string) (searchResults, error) {
	safeAuthor := url.QueryEscape(author)
	url := fmt.Sprintf("https://openlibrary.org/search.json?author=%s", safeAuthor)
	resp, err := http.Get(url)
	if err != nil {
		// log and exit if there is an error fetching books
		log.Fatalf("Error fetching data: %v", err)
	}
	defer resp.Body.Close()
	var results searchResults
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		log.Fatalf("Error decoding data: %v", err)
	}

	if len(results.Docs) == 0 {
		fmt.Println("No books found for ", author)
	}

	fmt.Println("Books found for ", author)
	for _, book := range results.Docs {
		fmt.Printf(" - %s\n", book.Title)
	}
	return results, nil
}

func main() {
	rootcmd.AddCommand(cmsSearch)
	if err := rootcmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
