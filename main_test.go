package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// RoundTripFunc type is an adapter to allow the use of ordinary functions as http.RoundTripper.
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip executes a single HTTP transaction and implements the http.RoundTripper interface.
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newResponse is a helper to create an http.Response with a given status and body.
func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func TestSearchHandler(t *testing.T) {
	// Save the original transport and restore it after tests.
	originalTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = originalTransport }()

	// Override the transport to simulate various responses based on the "author" parameter.
	http.DefaultClient.Transport = RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		author := req.URL.Query().Get("author")
		switch author {
		case "error":
			// Simulate a network error.
			return nil, errors.New("simulated network error")
		case "badjson":
			// Simulate an API returning malformed JSON.
			return newResponse(200, "not json"), nil
		case "nobooks":
			// Simulate an API returning no books.
			return newResponse(200, `{"docs": []}`), nil
		case "someauthor":
			// Simulate a successful API response with one book.
			return newResponse(200, `{"docs": [{"title": "Test Book"}]}`), nil
		default:
			// Default successful response.
			return newResponse(200, `{"docs": [{"title": "Default Book"}]}`), nil
		}
	})

	// Table-driven tests.
	tests := []struct {
		name                  string
		query                 string // value for "author" query parameter
		expectedStatus        int
		expectedBodySubstring string
	}{
		{
			name:                  "Missing author",
			query:                 "",
			expectedStatus:        http.StatusBadRequest,
			expectedBodySubstring: "Missing 'author' query parameter",
		},
		{
			name:                  "HTTP GET error",
			query:                 "error",
			expectedStatus:        http.StatusInternalServerError,
			expectedBodySubstring: "Error fetching data:",
		},
		{
			name:                  "Malformed JSON response",
			query:                 "badjson",
			expectedStatus:        http.StatusInternalServerError,
			expectedBodySubstring: "Error decoding data:",
		},
		{
			name:                  "No books found",
			query:                 "nobooks",
			expectedStatus:        http.StatusNotFound,
			expectedBodySubstring: "No books found for author",
		},
		{
			name:                  "Successful search",
			query:                 "someauthor",
			expectedStatus:        http.StatusOK,
			expectedBodySubstring: "Test Book",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new HTTP request to our handler.
			req := httptest.NewRequest("GET", "/search", nil)
			q := req.URL.Query()
			if tc.query != "" {
				q.Add("author", tc.query)
			}
			req.URL.RawQuery = q.Encode()

			// Create a ResponseRecorder to capture the handler's response.
			rr := httptest.NewRecorder()

			// Call the handler.
			searchHandler(rr, req)

			// Check the response status code.
			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}

			// Check that the response body contains the expected substring.
			if !strings.Contains(rr.Body.String(), tc.expectedBodySubstring) {
				t.Errorf("expected response body to contain %q, got %q", tc.expectedBodySubstring, rr.Body.String())
			}
		})
	}
}
