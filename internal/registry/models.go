package registry

import "time"

type Domain struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Active    bool      `json:"active"`
}

type Record struct {
	ID       int64  `json:"id"`
	DomainID int64  `json:"domain_id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

type Site struct {
	ID          int64     `json:"id"`
	DomainID    int64     `json:"domain_id"`
	DomainName  string    `json:"domain_name"`
	Type        string    `json:"type"`
	Target      string    `json:"target"`
	HTTPSEnabled bool     `json:"https_enabled"`
	CreatedAt   time.Time `json:"created_at"`
}
