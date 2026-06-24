package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"dns-server/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}
	apiURL := fmt.Sprintf("http://localhost:%d/api/v1", cfg.HTTPSPort+1)

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		return
	}

	cmd := args[0]
	switch cmd {
	case "domain":
		handleDomain(args[1:], apiURL, cfg.AdminToken)
	case "record":
		handleRecord(args[1:], apiURL, cfg.AdminToken)
	case "site":
		handleSite(args[1:], apiURL, cfg.AdminToken)
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Print(`usage: dns <command> [args]

commands:
  domain list                                          list domains
  domain register <name>                               register a domain
  domain delete <name>                                 delete a domain

  record list <domain>                                 list records for a domain
  record add <domain> <type> <name> <value> [ttl]      add a DNS record

  site list                                            list hosted sites
  site add <domain> <type> [target] [--https]          add a site (type: static|proxy|custom)
  site delete <domain>                                 remove a site
`)
}

func handleDomain(args []string, apiURL, token string) {
	if len(args) < 1 {
		fmt.Println("usage: dns domain <list|register|delete>")
		return
	}
	sub := args[0]
	switch sub {
	case "list":
		resp := doRequest("GET", apiURL+"/domains", nil, token)
		fmt.Println(string(resp))
	case "register":
		if len(args) < 2 {
			fmt.Println("usage: dns domain register <name>")
			return
		}
		body := map[string]string{"name": args[1]}
		data, _ := json.Marshal(body)
		resp := doRequest("POST", apiURL+"/domains", bytes.NewReader(data), token)
		fmt.Println(string(resp))
	case "delete":
		if len(args) < 2 {
			fmt.Println("usage: dns domain delete <name>")
			return
		}
		doRequest("DELETE", apiURL+"/domains/"+args[1], nil, token)
		fmt.Println("deleted")
	}
}

func handleRecord(args []string, apiURL, token string) {
	if len(args) < 1 {
		fmt.Println("usage: dns record <list|add|delete>")
		return
	}
	sub := args[0]
	switch sub {
	case "list":
		if len(args) < 2 {
			fmt.Println("usage: dns record list <domain>")
			return
		}
		resp := doRequest("GET", apiURL+"/domains/"+args[1]+"/records", nil, token)
		fmt.Println(string(resp))
	case "add":
		if len(args) < 5 {
			fmt.Println("usage: dns record add <domain> <type> <name> <value> [ttl]")
			return
		}
		body := map[string]interface{}{
			"type":  args[2],
			"name":  args[3],
			"value": args[4],
			"ttl":   300,
		}
		if len(args) > 5 {
			var ttl int
			fmt.Sscanf(args[5], "%d", &ttl)
			body["ttl"] = ttl
		}
		data, _ := json.Marshal(body)
		resp := doRequest("POST", apiURL+"/domains/"+args[1]+"/records", bytes.NewReader(data), token)
		fmt.Println(string(resp))
	}
}

func handleSite(args []string, apiURL, token string) {
	if len(args) < 1 {
		fmt.Println("usage: dns site <list|add|delete>")
		return
	}
	sub := args[0]
	switch sub {
	case "list":
		resp := doRequest("GET", apiURL+"/sites", nil, token)
		fmt.Println(string(resp))
	case "add":
		if len(args) < 3 {
			fmt.Println("usage: dns site add <domain> <static|proxy|custom> [target] [--https]")
			return
		}
		domain := args[1]
		siteType := args[2]
		target := ""
		https := false
		for _, a := range args[3:] {
			if a == "--https" {
				https = true
			} else {
				target = a
			}
		}
		body := map[string]interface{}{
			"domain_name":   domain,
			"type":          siteType,
			"target":        target,
			"https_enabled": https,
		}
		data, _ := json.Marshal(body)
		resp := doRequest("POST", apiURL+"/sites", bytes.NewReader(data), token)
		fmt.Println(string(resp))
	case "delete":
		if len(args) < 2 {
			fmt.Println("usage: dns site delete <domain>")
			return
		}
		doRequest("DELETE", apiURL+"/sites/"+args[1], nil, token)
		fmt.Println("deleted")
	default:
		fmt.Println("unknown subcommand")
	}
}

func doRequest(method, url string, body io.Reader, token string) []byte {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "error %d: %s\n", resp.StatusCode, strings.TrimSpace(string(data)))
		os.Exit(1)
	}
	return data
}
