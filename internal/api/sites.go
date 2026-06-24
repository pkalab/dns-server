package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type siteRequest struct {
	DomainName   string `json:"domain_name"`
	Type         string `json:"type"`
	Target       string `json:"target"`
	HTTPSEnabled bool   `json:"https_enabled"`
}

func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	sites, err := s.store.ListSites()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, sites)
}

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var req siteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.DomainName = strings.ToLower(strings.TrimSpace(req.DomainName))
	if req.DomainName == "" {
		writeError(w, "domain name required", http.StatusBadRequest)
		return
	}
	if !domainRegex.MatchString(req.DomainName) {
		writeError(w, "invalid domain name format", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = "static"
	}
	if req.Type != "static" && req.Type != "proxy" && req.Type != "custom" {
		writeError(w, "type must be static, proxy, or custom", http.StatusBadRequest)
		return
	}
	if req.Type == "proxy" && req.Target == "" {
		writeError(w, "target required for proxy type", http.StatusBadRequest)
		return
	}

	domain, err := s.store.GetDomain(req.DomainName)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if domain == nil {
		writeError(w, "domain not found, register it first", http.StatusNotFound)
		return
	}

	existing, _ := s.store.GetSite(req.DomainName)
	if existing != nil {
		writeError(w, "site already exists for this domain", http.StatusConflict)
		return
	}

	site, err := s.store.CreateSite(domain.ID, req.DomainName, req.Type, req.Target, req.HTTPSEnabled)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, site)
}

func (s *Server) deleteSite(w http.ResponseWriter, r *http.Request) {
	domainName := strings.ToLower(chi.URLParam(r, "domain"))
	if err := s.store.DeleteSite(domainName); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
