package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"dns-server/internal/registry"

	"github.com/go-chi/chi/v5"
)

var domainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*\.[a-z]{2,}$`)

type domainRequest struct {
	Name string `json:"name"`
}

type recordRequest struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
}

func (s *Server) listDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := s.store.ListDomains()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, domains)
}

func (s *Server) createDomain(w http.ResponseWriter, r *http.Request) {
	var req domainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if req.Name == "" {
		writeError(w, "domain name required", http.StatusBadRequest)
		return
	}
	if !domainRegex.MatchString(req.Name) {
		writeError(w, "invalid domain name format", http.StatusBadRequest)
		return
	}

	existing, _ := s.store.GetDomain(req.Name)
	if existing != nil {
		writeError(w, "domain already exists", http.StatusConflict)
		return
	}

	domain, err := s.store.CreateDomain(req.Name)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, domain)
}

func (s *Server) deleteDomain(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(chi.URLParam(r, "name"))
	if err := s.store.DeleteDomain(name); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listRecords(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(chi.URLParam(r, "name"))
	domain, err := s.store.GetDomain(name)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if domain == nil {
		writeError(w, "domain not found", http.StatusNotFound)
		return
	}

	records, err := s.store.GetRecords(domain.ID)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, records)
}

func (s *Server) addRecord(w http.ResponseWriter, r *http.Request) {
	name := strings.ToLower(chi.URLParam(r, "name"))
	domain, err := s.store.GetDomain(name)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if domain == nil {
		writeError(w, "domain not found", http.StatusNotFound)
		return
	}

	var req recordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Type = strings.ToUpper(req.Type)
	req.Name = strings.ToLower(req.Name)

	validTypes := map[string]bool{"A": true, "AAAA": true, "CNAME": true, "MX": true, "NS": true, "TXT": true}
	if !validTypes[req.Type] {
		writeError(w, "invalid record type", http.StatusBadRequest)
		return
	}
	if req.TTL == 0 {
		req.TTL = 300
	}

	record, err := s.store.AddRecord(domain.ID, req.Type, req.Name, req.Value, req.TTL, req.Priority)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, record)
}

func (s *Server) deleteRecord(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, "invalid record id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteRecord(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var _ = registry.Domain{}
