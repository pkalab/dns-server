package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dns-server/internal/api"
	"dns-server/internal/config"
	"dns-server/internal/dns"
	"dns-server/internal/registry"
	"dns-server/internal/webhost"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	store, err := registry.NewStore(cfg.Storage)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer store.Close()

	dnsStore := &dnsStoreAdapter{store: store}

	dnsServer := dns.NewServer(dnsStore, cfg.UpstreamDNS, cfg.DNSPort)
	go func() {
		if err := dnsServer.Start(); err != nil {
			log.Fatalf("dns server error: %v", err)
		}
	}()

	webServer := webhost.NewServer(store, cfg.SitesDir, cfg.HTTPPort, cfg.HTTPSPort, cfg.LetsEncryptEmail, cfg.LetsEncryptDir)
	go func() {
		if err := webServer.Start(); err != nil {
			log.Fatalf("web server error: %v", err)
		}
	}()

	apiServer := api.NewServer(store, cfg.AdminToken)

	adminAddr := fmt.Sprintf("127.0.0.1:%d", cfg.HTTPSPort+1)
	adminSrv := &http.Server{
		Addr:         adminAddr,
		Handler:      apiServer.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("[admin] API listening on %s", adminAddr)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("admin api error: %v", err)
		}
	}()

	fmt.Println("server started. press Ctrl+C to stop.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := adminSrv.Shutdown(ctx); err != nil {
		log.Printf("admin api shutdown error: %v", err)
	}
}

type dnsStoreAdapter struct {
	store *registry.Store
}

func (a *dnsStoreAdapter) GetDomain(name string) (*dns.Domain, error) {
	d, err := a.store.GetDomain(name)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	return &dns.Domain{ID: d.ID, Name: d.Name, Active: d.Active}, nil
}

func (a *dnsStoreAdapter) ListDomains() ([]dns.Domain, error) {
	domains, err := a.store.ListDomains()
	if err != nil {
		return nil, err
	}
	var result []dns.Domain
	for _, d := range domains {
		result = append(result, dns.Domain{ID: d.ID, Name: d.Name, Active: d.Active})
	}
	return result, nil
}

func (a *dnsStoreAdapter) GetRecords(domainID int64) ([]dns.Record, error) {
	records, err := a.store.GetRecords(domainID)
	if err != nil {
		return nil, err
	}
	var result []dns.Record
	for _, r := range records {
		result = append(result, dns.Record{
			DomainID: r.DomainID,
			Type:     r.Type,
			Name:     r.Name,
			Value:    r.Value,
			TTL:      r.TTL,
			Priority: r.Priority,
		})
	}
	return result, nil
}
