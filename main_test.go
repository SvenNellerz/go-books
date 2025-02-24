package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
)

// RoundTripFunc is an adapter to allow the use of ordinary functions as http.RoundTripper.
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

// TestLoginHandler tests the /login endpoint.
func TestLoginHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/login?username=test&password=test", nil)
	rr := httptest.NewRecorder()
	loginHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "token") {
		t.Errorf("expected token in response, got %q", rr.Body.String())
	}
}

// TestVulnerableHandler tests the /vulnerable endpoint.
func TestVulnerableHandler(t *testing.T) {
	message := "<script>alert('xss')</script>"
	req := httptest.NewRequest("GET", "/vulnerable?message="+url.QueryEscape(message), nil)
	rr := httptest.NewRecorder()
	vulnerableHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), message) {
		t.Errorf("expected response body to contain %q, got %q", message, rr.Body.String())
	}
}

// generateTestToken creates a valid JWT token for testing.
func generateTestToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": "testuser",
		"iat":      time.Now().Unix(),
		// Note: no expiration claim for testing (as per the vulnerable example)
	})
	return token.SignedString(jwtSecret)
}

// TestSearchHandler tests the /api/search endpoint which is protected by JWT and rate-limiting middleware.
func TestSearchHandler(t *testing.T) {
	// Save the original transport and restore it after tests.
	originalTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = originalTransport }()

	// Override the transport to simulate various responses based on the "author" parameter.
	http.DefaultClient.Transport = RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		author := req.URL.Query().Get("author")
		switch author {
		case "error":
			return nil, errors.New("simulated network error")
		case "badjson":
			return newResponse(200, "not json"), nil
		case "nobooks":
			return newResponse(200, `{"docs": []}`), nil
		case "someauthor":
			return newResponse(200, `{"docs": [{"title": "Test Book"}]}`), nil
		default:
			return newResponse(200, `{"docs": [{"title": "Default Book"}]}`), nil
		}
	})

	// Set up a router mimicking the main application's routing.
	router := mux.NewRouter()
	api := router.PathPrefix("/api").Subrouter()
	api.Use(jwtMiddleware)
	api.Handle("/search", rateLimitMiddleware(http.HandlerFunc(searchHandler))).Methods("GET")

	// Generate a valid token for protected endpoints.
	token, err := generateTestToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Table-driven tests.
	tests := []struct {
		name                  string
		query                 string // value for "author" query parameter
		tokenProvided         bool   // whether the request includes a valid token
		expectedStatus        int
		expectedBodySubstring string
	}{
		{
			name:                  "Missing token",
			query:                 "someauthor",
			tokenProvided:         false,
			expectedStatus:        http.StatusUnauthorized,
			expectedBodySubstring: "Missing Authorization header",
		},
		{
			name:                  "Missing author",
			query:                 "",
			tokenProvided:         true,
			expectedStatus:        http.StatusBadRequest,
			expectedBodySubstring: "Missing 'author' query parameter",
		},
		{
			name:                  "HTTP GET error",
			query:                 "error",
			tokenProvided:         true,
			expectedStatus:        http.StatusInternalServerError,
			expectedBodySubstring: "Error fetching data:",
		},
		{
			name:                  "Malformed JSON response",
			query:                 "badjson",
			tokenProvided:         true,
			expectedStatus:        http.StatusInternalServerError,
			expectedBodySubstring: "Error decoding data:",
		},
		{
			name:                  "No books found",
			query:                 "nobooks",
			tokenProvided:         true,
			expectedStatus:        http.StatusNotFound,
			expectedBodySubstring: "No books found for author",
		},
		{
			name:                  "Successful search",
			query:                 "someauthor",
			tokenProvided:         true,
			expectedStatus:        http.StatusOK,
			expectedBodySubstring: "Test Book",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/search", nil)
			q := req.URL.Query()
			if tc.query != "" {
				q.Add("author", tc.query)
			}
			req.URL.RawQuery = q.Encode()

			if tc.tokenProvided {
				req.Header.Add("Authorization", "Bearer "+token)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), tc.expectedBodySubstring) {
				t.Errorf("expected response body to contain %q, got %q", tc.expectedBodySubstring, rr.Body.String())
			}
		})
	}
}
