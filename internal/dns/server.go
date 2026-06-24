package dns

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type Server struct {
	store    Store
	upstream string
	addr     string
}

type Store interface {
	GetDomain(name string) (*Domain, error)
	GetRecords(domainID int64) ([]Record, error)
	ListDomains() ([]Domain, error)
}

type Domain struct {
	ID     int64
	Name   string
	Active bool
}

type Record struct {
	DomainID int64
	Type     string
	Name     string
	Value    string
	TTL      int
	Priority int
}

func NewServer(store Store, upstream string, port int) *Server {
	return &Server{
		store:    store,
		upstream: upstream,
		addr:     fmt.Sprintf(":%d", port),
	}
}

func (s *Server) Start() error {
	handler := NewHandler(s.store, s.upstream)

	udpServer := &dns.Server{
		Addr:    s.addr,
		Net:     "udp",
		Handler: handler,
	}
	tcpServer := &dns.Server{
		Addr:    s.addr,
		Net:     "tcp",
		Handler: handler,
	}

	errCh := make(chan error, 2)

	go func() {
		fmt.Printf("[dns] listening UDP on %s\n", s.addr)
		if err := udpServer.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("udp dns server: %w", err)
		}
	}()

	go func() {
		fmt.Printf("[dns] listening TCP on %s\n", s.addr)
		if err := tcpServer.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("tcp dns server: %w", err)
		}
	}()

	return <-errCh
}

func (s *Server) Addr() string {
	return s.addr
}

type Handler struct {
	store    Store
	upstream string
}

func NewHandler(store Store, upstream string) *Handler {
	return &Handler{store: store, upstream: upstream}
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)
	m.Authoritative = true

	if len(req.Question) == 0 {
		w.WriteMsg(m)
		return
	}

	q := req.Question[0]
	qname := strings.TrimSuffix(strings.ToLower(q.Name), ".")

	domain := h.findDomain(qname)
	if domain == nil {
		m.Authoritative = false
		m.RecursionAvailable = true
		resp, err := resolveUpstream(q, h.upstream)
		if err != nil {
			m.Rcode = dns.RcodeServerFailure
			w.WriteMsg(m)
			return
		}
		m.Answer = resp
		w.WriteMsg(m)
		return
	}

	records, err := h.store.GetRecords(domain.ID)
	if err != nil {
		m.Rcode = dns.RcodeServerFailure
		w.WriteMsg(m)
		return
	}

	subdomain := strings.TrimSuffix(qname, "."+domain.Name)

	if qname == domain.Name {
		subdomain = "@"
	}

	for _, rec := range records {
		recName := strings.ToLower(rec.Name)
		if recName != subdomain && recName != "*" {
			continue
		}
		if uint16(dns.StringToType[rec.Type]) != q.Qtype && q.Qtype != dns.TypeANY {
			continue
		}

		hdr := dns.RR_Header{
			Name:   q.Name,
			Rrtype: dns.StringToType[rec.Type],
			Class:  dns.ClassINET,
			Ttl:    uint32(rec.TTL),
		}
		var rr dns.RR
		switch rec.Type {
		case "A":
			rr = &dns.A{Hdr: hdr, A: net.ParseIP(rec.Value)}
		case "AAAA":
			rr = &dns.AAAA{Hdr: hdr, AAAA: net.ParseIP(rec.Value)}
		case "CNAME":
			rr = &dns.CNAME{Hdr: hdr, Target: rec.Value}
		case "MX":
			rr = &dns.MX{Hdr: hdr, Mx: rec.Value, Preference: uint16(rec.Priority)}
		case "NS":
			rr = &dns.NS{Hdr: hdr, Ns: rec.Value}
		case "TXT":
			rr = &dns.TXT{Hdr: hdr, Txt: []string{rec.Value}}
		default:
			continue
		}
		m.Answer = append(m.Answer, rr)
	}

	w.WriteMsg(m)
}

func (h *Handler) findDomain(qname string) *Domain {
	parts := strings.Split(qname, ".")
	for i := 0; i < len(parts); i++ {
		candidate := strings.Join(parts[i:], ".")
		domain, _ := h.store.GetDomain(candidate)
		if domain != nil && domain.Active {
			return domain
		}
	}
	return nil
}

func resolveUpstream(question dns.Question, upstream string) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(question.Name, question.Qtype)
	m.RecursionDesired = true

	client := new(dns.Client)
	resp, _, err := client.Exchange(m, upstream)
	if err != nil {
		return nil, err
	}

	if len(resp.Answer) > 0 {
		return resp.Answer, nil
	}
	return resp.Extra, nil
}
