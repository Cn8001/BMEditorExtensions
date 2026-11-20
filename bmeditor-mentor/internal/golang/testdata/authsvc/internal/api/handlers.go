// Package api serves the HTTP API.
package api

import (
	"encoding/json"
	"net/http"

	"example.com/authsvc/internal/auth"
)

// LoginHandler answers sign-in requests.
type LoginHandler struct{ Auth *auth.Service }

// ServeLogin signs a user in with the provider and code in the request.
func (h *LoginHandler) ServeLogin(w http.ResponseWriter, r *http.Request) {
	sess, err := h.Auth.Login(r.Context(), r.FormValue("provider"), r.FormValue("code"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"token": sess.Token})
}
