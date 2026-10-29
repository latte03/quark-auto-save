// Package source ports the CloudSaver and PanSou share search clients.
package source

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Result struct {
	ShareURL string   `json:"shareurl"`
	TaskName string   `json:"taskname"`
	Content  string   `json:"content"`
	Datetime string   `json:"datetime"`
	Channel  string   `json:"channel"`
	Source   string   `json:"source"`
	Tags     []string `json:"tags,omitempty"`
}

type CloudSaverConfig struct {
	Enable   bool   `json:"enable"`
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

type PanSouConfig struct {
	Enable bool   `json:"enable"`
	Server string `json:"server"`
}

type Options struct {
	CloudSaver CloudSaverConfig
	PanSou     PanSouConfig
	HTTP       *http.Client
}

func (o Options) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

// Search queries every enabled source in parallel and merges the results.
// It also returns partial-failure messages and a refreshed CloudSaver token.
func Search(keyword string, deep bool, opts Options) ([]Result, []string, string) {
	type outcome struct {
		source  string
		results []Result
		message string
		token   string
	}
	results := make(chan outcome, 2)
	pending := 0
	if opts.PanSou.Enable && opts.PanSou.Server != "" {
		pending++
		go func() {
			items, err := searchPanSou(opts, keyword, deep)
			message := ""
			if err != nil {
				message = "PanSou: " + err.Error()
			}
			results <- outcome{source: "PanSou", results: items, message: message}
		}()
	}
	if opts.CloudSaver.Enable && opts.CloudSaver.Server != "" {
		pending++
		go func() {
			items, token, err := searchCloudSaver(opts, keyword)
			message := ""
			if err != nil {
				message = "CloudSaver: " + err.Error()
			}
			results <- outcome{source: "CloudSaver", results: items, message: message, token: token}
		}()
	}
	// Collect first, then merge in a fixed source order so duplicate links
	// always resolve to the same source regardless of goroutine timing.
	collected := map[string]outcome{}
	for i := 0; i < pending; i++ {
		item := <-results
		collected[item.source] = item
	}
	var merged []Result
	var messages []string
	newToken := ""
	seen := map[string]bool{}
	for _, name := range []string{"CloudSaver", "PanSou"} {
		item, ok := collected[name]
		if !ok {
			continue
		}
		if item.message != "" {
			messages = append(messages, item.message)
		}
		if item.token != "" {
			newToken = item.token
		}
		for _, result := range item.results {
			if result.ShareURL == "" || seen[result.ShareURL] {
				continue
			}
			seen[result.ShareURL] = true
			merged = append(merged, result)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Datetime > merged[j].Datetime
	})
	return merged, messages, newToken
}

type cloudSaverClient struct {
	cfg  CloudSaverConfig
	http *http.Client
}

func (c *cloudSaverClient) request(method, endpoint string, query url.Values, body any, token string) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = strings.NewReader(string(encoded))
	}
	request, err := http.NewRequest(method, strings.TrimRight(c.cfg.Server, "/")+endpoint, reader)
	if err != nil {
		return nil, err
	}
	request.URL.RawQuery = query.Encode()
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", response.StatusCode)
	}
	return raw, nil
}

func (c *cloudSaverClient) search(keyword, token string) ([]byte, error) {
	return c.request(http.MethodGet, "/api/search", url.Values{
		"keyword":       {keyword},
		"lastMessageId": {""},
	}, nil, token)
}

func (c *cloudSaverClient) login() (string, error) {
	raw, err := c.request(http.MethodPost, "/api/user/login", nil, map[string]string{
		"username": c.cfg.Username,
		"password": c.cfg.Password,
	}, "")
	if err != nil {
		return "", err
	}
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", err
	}
	if !response.Success || response.Data.Token == "" {
		return "", fmt.Errorf("登录失败: %s", response.Message)
	}
	return response.Data.Token, nil
}

func (c *cloudSaverClient) searchOrLogin(keyword string) ([]Result, string, error) {
	token := c.cfg.Token
	raw, err := c.search(keyword, token)
	if err != nil {
		return nil, "", err
	}
	results, message, ok := parseCloudSaver(raw)
	if ok {
		return results, "", nil
	}
	if message != "无效的 token" && message != "未提供 token" {
		return nil, "", fmt.Errorf("%s", message)
	}
	token, err = c.login()
	if err != nil {
		return nil, "", err
	}
	raw, err = c.search(keyword, token)
	if err != nil {
		return nil, "", err
	}
	results, message, ok = parseCloudSaver(raw)
	if !ok {
		return nil, "", fmt.Errorf("%s", message)
	}
	return results, token, nil
}

func parseCloudSaver(raw []byte) ([]Result, string, bool) {
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    []struct {
			List []struct {
				Title     string   `json:"title"`
				Content   string   `json:"content"`
				PubDate   string   `json:"pubDate"`
				ChannelID string   `json:"channelId"`
				Tags      []string `json:"tags"`
				Links     []struct {
					CloudType string `json:"cloudType"`
					Link      string `json:"link"`
				} `json:"cloudLinks"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, "响应解析失败", false
	}
	if !response.Success {
		return nil, response.Message, false
	}
	titleRe := regexp.MustCompile(`(?s)(名称|标题)[：:]?(.*)`)
	contentRe := regexp.MustCompile(`(?s)(描述|简介)[：:]?(.*)(链接|标签)`)
	var results []Result
	seen := map[string]bool{}
	for _, channel := range response.Data {
		for _, item := range channel.List {
			for _, link := range item.Links {
				if link.CloudType != "quark" || link.Link == "" || seen[link.Link] {
					continue
				}
				seen[link.Link] = true
				title := item.Title
				if match := titleRe.FindStringSubmatch(title); match != nil {
					title = match[2]
				}
				title = strings.TrimSpace(strings.ReplaceAll(title, "&amp;", "&"))
				content := item.Content
				if match := contentRe.FindStringSubmatch(content); match != nil {
					content = match[2]
				}
				content = strings.ReplaceAll(content, `<mark class="highlight">`, "")
				content = strings.TrimSpace(strings.ReplaceAll(content, "</mark>", ""))
				results = append(results, Result{
					ShareURL: link.Link,
					TaskName: title,
					Content:  content,
					Datetime: isoToCST(item.PubDate),
					Channel:  item.ChannelID,
					Source:   "CloudSaver",
					Tags:     item.Tags,
				})
			}
		}
	}
	return results, "", true
}

func searchCloudSaver(opts Options, keyword string) ([]Result, string, error) {
	client := &cloudSaverClient{cfg: opts.CloudSaver, http: opts.client()}
	return client.searchOrLogin(keyword)
}

type panSouClient struct {
	server string
	http   *http.Client
}

func searchPanSou(opts Options, keyword string, refresh bool) ([]Result, error) {
	client := &panSouClient{server: strings.TrimRight(opts.PanSou.Server, "/"), http: opts.client()}
	query := url.Values{
		"kw":          {keyword},
		"cloud_types": {"quark"},
		"res":         {"merge"},
		"refresh":     {fmt.Sprint(refresh)},
	}
	response, err := client.http.Get(client.server + "/api/search?" + query.Encode())
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d", response.StatusCode)
	}
	return parsePanSou(raw)
}

func parsePanSou(raw []byte) ([]Result, error) {
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Merged struct {
				Quark []struct {
					URL      string `json:"url"`
					Note     string `json:"note"`
					Datetime string `json:"datetime"`
					Source   string `json:"source"`
				} `json:"quark"`
			} `json:"merged_by_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	noteRe := regexp.MustCompile(`(?s)^(.*?)(?:[【\[]?(?:简介|介绍|描述)[】\]]?[:：]?)(.*)$`)
	var results []Result
	seen := map[string]bool{}
	for _, item := range payload.Data.Merged.Quark {
		if item.URL == "" || seen[item.URL] {
			continue
		}
		seen[item.URL] = true
		title, content := item.Note, ""
		if match := noteRe.FindStringSubmatch(item.Note); match != nil {
			title, content = match[1], match[2]
		}
		results = append(results, Result{
			ShareURL: item.URL,
			TaskName: strings.TrimSpace(title),
			Content:  strings.TrimSpace(content),
			Datetime: isoToCST(item.Datetime),
			Channel:  item.Source,
			Source:   "PanSou",
		})
	}
	return results, nil
}

// isoToCST mirrors sdk/common.py: parse ISO timestamps and render China time.
func isoToCST(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		// Naive timestamps come from China-hosted sources; treat them as CST
		// instead of UTC so the rendered time does not shift by 8 hours.
		cst := time.FixedZone("CST", 8*3600)
		if fallback, fallbackErr := time.ParseInLocation("2006-01-02T15:04:05", value, cst); fallbackErr == nil {
			parsed = fallback
		} else {
			return ""
		}
	}
	cst := parsed.In(time.FixedZone("CST", 8*3600))
	if cst.Year() < 1970 {
		return ""
	}
	return cst.Format("2006-01-02 15:04:05")
}
