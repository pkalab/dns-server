package webhost

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dns-server/internal/registry"
	"golang.org/x/crypto/acme/autocert"
)

type Server struct {
	store       *registry.Store
	sitesDir    string
	httpPort    int
	httpsPort   int
	leEmail     string
	leDir       string
}

func NewServer(store *registry.Store, sitesDir string, httpPort, httpsPort int, leEmail, leDir string) *Server {
	return &Server{
		store:     store,
		sitesDir:  sitesDir,
		httpPort:  httpPort,
		httpsPort: httpsPort,
		leEmail:   leEmail,
		leDir:     leDir,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.vhostHandler)

	var tlsConfig *tls.Config
	if s.leEmail != "" {
		certManager := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Email:      s.leEmail,
			Cache:      autocert.DirCache(s.leDir),
			HostPolicy: s.hostPolicy,
		}
		tlsConfig = &tls.Config{
			GetCertificate: certManager.GetCertificate,
		}
	}

	errCh := make(chan error, 2)

	go func() {
		addr := fmt.Sprintf(":%d", s.httpPort)
		fmt.Printf("[web] listening HTTP on %s\n", addr)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.leEmail != "" && r.TLS == nil {
				host := s.extractHost(r.Host)
				if !s.isKnownSite(host) {
					http.NotFound(w, r)
					return
				}
				target := "https://" + host + r.URL.RequestURI()
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			mux.ServeHTTP(w, r)
		})
		srv := &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	if s.leEmail != "" {
		go func() {
			addr := fmt.Sprintf(":%d", s.httpsPort)
			fmt.Printf("[web] listening HTTPS on %s\n", addr)
			srv := &http.Server{
				Addr:         addr,
				Handler:      securityHeadersMiddleware(mux),
				TLSConfig:    tlsConfig,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  60 * time.Second,
			}
			if err := srv.ListenAndServeTLS("", ""); err != nil {
				errCh <- fmt.Errorf("https server: %w", err)
			}
		}()
	}

	return <-errCh
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) extractHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(host)
	}
	return host
}

func (s *Server) isKnownSite(host string) bool {
	site, err := s.store.GetSite(host)
	return err == nil && site != nil
}

func (s *Server) hostPolicy(ctx context.Context, host string) error {
	host = s.extractHost(host)
	site, err := s.store.GetSite(host)
	if err != nil {
		return err
	}
	if site == nil {
		return fmt.Errorf("unknown host: %s", host)
	}
	if !site.HTTPSEnabled {
		return fmt.Errorf("HTTPS not enabled for: %s", host)
	}
	return nil
}

func (s *Server) vhostHandler(w http.ResponseWriter, r *http.Request) {
	host := s.extractHost(r.Host)

	site, err := s.store.GetSite(host)
	if err != nil || site == nil {
		http.NotFound(w, r)
		return
	}

	switch site.Type {
	case "static":
		sitePath := filepath.Join(s.sitesDir, site.DomainName)
		absPath, err := filepath.Abs(sitePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		absBase, err := filepath.Abs(s.sitesDir)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(absPath, absBase) {
			http.NotFound(w, r)
			return
		}
		fs := http.FileServer(http.Dir(sitePath))
		http.StripPrefix("/", fs).ServeHTTP(w, r)
	case "proxy":
		proxyTo := site.Target
		if !strings.HasPrefix(proxyTo, "http://") && !strings.HasPrefix(proxyTo, "https://") {
			http.Error(w, "invalid proxy target", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, proxyTo+r.URL.Path, http.StatusTemporaryRedirect)
	case "custom":
		data, err := os.ReadFile(filepath.Join(s.sitesDir, site.DomainName, "index.html"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	default:
		http.NotFound(w, r)
	}
}
