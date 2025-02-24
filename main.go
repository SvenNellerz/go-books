package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// NOTE: This code intentionally includes vulnerabilities for demonstration purposes only.

// Book struct to hold book data.
type Book struct {
	Title string `json:"title"`
}

// searchResults holds the API response structure.
type searchResults struct {
	Docs []Book `json:"docs"`
}

// jwtSecret is a hardcoded secret key (vulnerable to exposure).
var jwtSecret = []byte("supersecretkey")

// rateLimiter is a simple in-memory rate limiter map that is NOT thread-safe.
// In a production system, use a robust rate limiter with proper locking or an external store.
var rateLimiter = make(map[string]int)

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

// loginHandler issues a JWT token for a user.
// Vulnerabilities:
// - Accepts credentials via query parameters (insecure).
// - Uses a hardcoded secret and no password validation.
// - Does not set token expiration.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")

	if username == "" || password == "" {
		http.Error(w, "Missing credentials", http.StatusBadRequest)
		return
	}

	// Insecure: any username/password is accepted.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"iat":      time.Now().Unix(),
		// No expiration claim added
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, fmt.Sprintf(`{"token": "%s"}`, tokenString))
}

// vulnerableHandler echoes a query parameter unsafely, making it vulnerable to XSS attacks.
func vulnerableHandler(w http.ResponseWriter, r *http.Request) {
	// Vulnerable: Echoing user input directly without sanitization.
	message := r.URL.Query().Get("message")
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><body><h1>User Message:</h1><p>%s</p></body></html>", message)
}

// jwtMiddleware protects routes by requiring a valid JWT token.
// Vulnerabilities:
// - Uses a hardcoded secret.
// - Does not check token expiration or additional claims.
func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		// Expected format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			// Vulnerable: Simply returns the hardcoded secret.
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware is a simple rate limiting middleware.
// Vulnerabilities:
// - The in-memory map is not protected from concurrent access.
// - The rate count is never reset.
func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if count, exists := rateLimiter[ip]; exists && count >= 10 {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		// Vulnerable to race conditions: concurrent requests may access rateLimiter unsafely.
		rateLimiter[ip]++
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Set up Logrus for logging.
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.Info("Starting application...")

	// Use Gorilla Mux router.
	router := mux.NewRouter()

	// Public endpoints.
	router.HandleFunc("/login", loginHandler).Methods("GET")
	router.Handle("/vulnerable", rateLimitMiddleware(http.HandlerFunc(vulnerableHandler))).Methods("GET")

	// Protected endpoints (require valid JWT).
	api := router.PathPrefix("/api").Subrouter()
	api.Use(jwtMiddleware)
	api.Handle("/search", rateLimitMiddleware(http.HandlerFunc(searchHandler))).Methods("GET")

	// Use the PORT environment variable if available, else default to 8080.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	logrus.Infof("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		logrus.Fatalf("Server failed: %v", err)
	}
}
