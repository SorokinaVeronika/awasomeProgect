package internal

import (
	"net/http"

	"github.com/gorilla/mux"
)

func MakeHTTPHandler(h *Handlers) http.Handler {
	r := mux.NewRouter()

	// Use the requireTokenAuthentication middleware for routes that require authentication
	secured := r.PathPrefix("/secured").Subrouter()
	secured.Use(h.RequireTokenAuthentication)

	secured.HandleFunc("/etfs", h.ListETFSymbolsHandler).Methods("GET")
	secured.HandleFunc("/etf/{ticker}", h.GetETFDataHandler).Methods("GET")

	r.HandleFunc("/login", h.LoginHandler).Methods("POST")

	return r
}
