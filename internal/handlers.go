package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"

	"awesomeProject/models"
)

type Handlers struct {
	server    *Server
	jwtSecret []byte
}

func NewHandler(server *Server, jwtSecret []byte) *Handlers {
	return &Handlers{
		server:    server,
		jwtSecret: jwtSecret,
	}
}

// LoginHandler function for user login and token generation
func (h Handlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	exists, err := h.server.UserExists(username, password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !exists {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, err := h.generateToken(username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	WriteJSONResponse(w, token)
}

// RequireTokenAuthentication middleware function for JWT authentication
func (h Handlers) RequireTokenAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &models.Claims{}, func(token *jwt.Token) (interface{}, error) {
			return h.jwtSecret, nil
		})
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(*models.Claims); ok && token.Valid {
			// Token is valid, proceed to the next handler
			r = r.WithContext(context.WithValue(r.Context(), "username", claims.Username))
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
}

// ListETFSymbolsHandler function for listing available ETF symbols
func (h Handlers) ListETFSymbolsHandler(w http.ResponseWriter, r *http.Request) {
	etf, err := h.server.GetAllTickers()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	WriteJSONResponse(w, etf)
}

// GetETFDataHandler function for getting ETF data by ticker
func (h Handlers) GetETFDataHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ticker := vars["ticker"]

	if ticker == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	etf, err := h.server.GetETF(ticker)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	WriteJSONResponse(w, etf)
}

// Define a function to generate JWT tokens
func (h Handlers) generateToken(username string) (string, error) {
	claims := models.Claims{
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: jwt.TimeFunc().Add(time.Hour * 24).Unix(), // Token expires in 24 hours
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}

func WriteJSONResponse(w http.ResponseWriter, data interface{}) {
	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Marshal the data into JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}

	// Write the JSON to the response writer
	_, err = w.Write(jsonData)
	if err != nil {
		http.Error(w, "Failed to write JSON response", http.StatusInternalServerError)
		return
	}
}
