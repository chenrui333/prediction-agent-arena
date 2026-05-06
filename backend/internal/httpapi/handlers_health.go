package httpapi

import "net/http"

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	redisOK := true
	if err := s.Cache.Ping(r.Context()); err != nil {
		redisOK = false
	}
	health, err := s.Store.Health(r.Context(), redisOK)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, health)
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) adminHealth(w http.ResponseWriter, r *http.Request) {
	s.health(w, r)
}
