package registry

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS domains (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			active BOOLEAN DEFAULT 1
		);
		CREATE TABLE IF NOT EXISTS records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			value TEXT NOT NULL,
			ttl INTEGER DEFAULT 300,
			priority INTEGER DEFAULT 0,
			FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
		);
		CREATE TABLE IF NOT EXISTS sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain_id INTEGER NOT NULL UNIQUE,
			domain_name TEXT NOT NULL,
			type TEXT DEFAULT 'static',
			target TEXT DEFAULT '',
			https_enabled BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
		);
	`)
	return err
}

func (s *Store) CreateDomain(name string) (*Domain, error) {
	now := time.Now()
	res, err := s.db.Exec(
		"INSERT INTO domains (name, created_at, updated_at, active) VALUES (?, ?, ?, 1)",
		name, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Domain{ID: id, Name: name, CreatedAt: now, UpdatedAt: now, Active: true}, nil
}

func (s *Store) ListDomains() ([]Domain, error) {
	rows, err := s.db.Query("SELECT id, name, created_at, updated_at, active FROM domains ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.CreatedAt, &d.UpdatedAt, &d.Active); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, nil
}

func (s *Store) GetDomain(name string) (*Domain, error) {
	d := &Domain{}
	err := s.db.QueryRow(
		"SELECT id, name, created_at, updated_at, active FROM domains WHERE name = ?",
		name,
	).Scan(&d.ID, &d.Name, &d.CreatedAt, &d.UpdatedAt, &d.Active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Store) GetDomainByID(id int64) (*Domain, error) {
	d := &Domain{}
	err := s.db.QueryRow(
		"SELECT id, name, created_at, updated_at, active FROM domains WHERE id = ?",
		id,
	).Scan(&d.ID, &d.Name, &d.CreatedAt, &d.UpdatedAt, &d.Active)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Store) DeleteDomain(name string) error {
	_, err := s.db.Exec("DELETE FROM domains WHERE name = ?", name)
	return err
}

func (s *Store) AddRecord(domainID int64, recType, name, value string, ttl, priority int) (*Record, error) {
	res, err := s.db.Exec(
		"INSERT INTO records (domain_id, type, name, value, ttl, priority) VALUES (?, ?, ?, ?, ?, ?)",
		domainID, recType, name, value, ttl, priority,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Record{ID: id, DomainID: domainID, Type: recType, Name: name, Value: value, TTL: ttl, Priority: priority}, nil
}

func (s *Store) GetRecords(domainID int64) ([]Record, error) {
	rows, err := s.db.Query(
		"SELECT id, domain_id, type, name, value, ttl, priority FROM records WHERE domain_id = ?",
		domainID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.DomainID, &r.Type, &r.Name, &r.Value, &r.TTL, &r.Priority); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func (s *Store) GetAllRecords() ([]Record, error) {
	rows, err := s.db.Query(
		"SELECT id, domain_id, type, name, value, ttl, priority FROM records ORDER BY domain_id",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.DomainID, &r.Type, &r.Name, &r.Value, &r.TTL, &r.Priority); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func (s *Store) DeleteRecord(id int64) error {
	_, err := s.db.Exec("DELETE FROM records WHERE id = ?", id)
	return err
}

func (s *Store) CreateSite(domainID int64, domainName, siteType, target string, httpsEnabled bool) (*Site, error) {
	now := time.Now()
	res, err := s.db.Exec(
		"INSERT INTO sites (domain_id, domain_name, type, target, https_enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		domainID, domainName, siteType, target, httpsEnabled, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Site{ID: id, DomainID: domainID, DomainName: domainName, Type: siteType, Target: target, HTTPSEnabled: httpsEnabled, CreatedAt: now}, nil
}

func (s *Store) GetSite(domainName string) (*Site, error) {
	st := &Site{}
	err := s.db.QueryRow(
		"SELECT id, domain_id, domain_name, type, target, https_enabled, created_at FROM sites WHERE domain_name = ?",
		domainName,
	).Scan(&st.ID, &st.DomainID, &st.DomainName, &st.Type, &st.Target, &st.HTTPSEnabled, &st.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) ListSites() ([]Site, error) {
	rows, err := s.db.Query("SELECT id, domain_id, domain_name, type, target, https_enabled, created_at FROM sites")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var st Site
		if err := rows.Scan(&st.ID, &st.DomainID, &st.DomainName, &st.Type, &st.Target, &st.HTTPSEnabled, &st.CreatedAt); err != nil {
			return nil, err
		}
		sites = append(sites, st)
	}
	return sites, nil
}

func (s *Store) DeleteSite(domainName string) error {
	_, err := s.db.Exec("DELETE FROM sites WHERE domain_name = ?", domainName)
	return err
}
