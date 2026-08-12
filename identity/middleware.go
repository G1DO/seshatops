package identity

import "net/http"

// RequireSession refuses requests that lack a fresh Go-owned session before
// invoking next. Unauthenticated callers receive 401 {"error":"unauthenticated"}.
func RequireSession(lookup SessionLookup, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lookup == nil {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthenticated"})
			return
		}
		sess, err := lookup.Session(r)
		if err != nil || sess == nil {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthenticated"})
			return
		}
		next.ServeHTTP(w, r.WithContext(WithSession(r.Context(), sess)))
	})
}
