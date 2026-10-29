package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultCloudSaverServer = "http://127.0.0.1:8008"
	defaultPanSouServer     = "http://127.0.0.1:50080"
)

// Alist uses 5244; OpenList, its fork, is commonly published on 9255 by fnOS.
var defaultAlistServers = []string{"http://127.0.0.1:5244", "http://127.0.0.1:9255"}

type sourceSuggestion struct {
	Server string `json:"server,omitempty"`
	Online bool   `json:"online"`
}

// handleDetectSources probes the default local ports used by CloudSaver and
// PanSou when they run on the same host as QAS.
func (s *Server) handleDetectSources(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}

	results := map[string]sourceSuggestion{
		"cloudsaver": {Online: false},
		"pansou":     {Online: false},
		"alist":      {Online: false},
	}
	var mu sync.Mutex
	var wg sync.WaitGroup

	check := func(name, server string, probe func(context.Context, *http.Client, string) bool) {
		defer wg.Done()
		if !probe(ctx, client, server) {
			return
		}
		mu.Lock()
		results[name] = sourceSuggestion{Server: server, Online: true}
		mu.Unlock()
	}

	wg.Add(3)
	go check("cloudsaver", defaultCloudSaverServer, probeCloudSaver)
	go check("pansou", defaultPanSouServer, probePanSou)
	go func() {
		defer wg.Done()
		for _, server := range defaultAlistServers {
			if probeAlist(ctx, client, server) {
				mu.Lock()
				results["alist"] = sourceSuggestion{Server: server, Online: true}
				mu.Unlock()
				return
			}
		}
	}()
	wg.Wait()

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": results})
}

func probeCloudSaver(ctx context.Context, client *http.Client, base string) bool {
	root, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err == nil {
		if response, err := client.Do(root); err == nil {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 256<<10))
			response.Body.Close()
			if response.StatusCode < 500 && strings.Contains(strings.ToLower(string(body)), "cloudsaver") {
				return true
			}
		}
	}

	probe, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/search?keyword=qas-probe", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(probe)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusForbidden {
		return false
	}
	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload); err != nil {
		return false
	}
	return payload["code"] != nil || payload["message"] != nil || payload["data"] != nil
}

func probePanSou(ctx context.Context, client *http.Client, base string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/search?kw=qas-probe&cloud_types=quark&res=merge", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var payload struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload); err != nil {
		return false
	}
	return payload.Code == 200 || len(payload.Data) > 0
}

func probeAlist(ctx context.Context, client *http.Client, base string) bool {
	if request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/ping", nil); err == nil {
		if response, err := client.Do(request); err == nil {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
			response.Body.Close()
			if response.StatusCode == http.StatusOK && strings.Contains(strings.ToLower(string(body)), "pong") {
				return true
			}
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode >= 500 {
		return false
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	text := strings.ToLower(string(body))
	return strings.Contains(text, "alist") || strings.Contains(text, "openlist")
}
