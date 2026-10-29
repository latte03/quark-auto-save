package quark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	BaseURL    = "https://drive-pc.quark.cn"
	BaseURLApp = "https://drive-m.quark.cn"
	userAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) quark-cloud-drive/3.14.2 Chrome/112.0.5615.165 Electron/24.1.3.8 Safari/537.36 Channel/pckk_other_ch"
)

type Client struct {
	cookie     string
	mparam     map[string]string
	http       *http.Client
	debug      bool
	baseURL    string
	baseURLApp string
	accountURL string
}

func New(cookie string) *Client {
	return &Client{
		cookie:     strings.TrimSpace(cookie),
		mparam:     parseMParam(cookie),
		http:       &http.Client{Timeout: 30 * time.Second},
		baseURL:    BaseURL,
		baseURLApp: BaseURLApp,
		accountURL: accountInfoURL,
	}
}

func (c *Client) SetBaseURL(base, app string) {
	c.baseURL = base
	c.baseURLApp = app
}

func (c *Client) SetDebug(v bool)       { c.debug = v }
func (c *Client) HasMobileParams() bool { return len(c.mparam) == 3 }

func (c *Client) CookieLength() int { return len(c.cookie) }

type envelope struct {
	Status   flexInt         `json:"status"`
	Code     flexInt         `json:"code"`
	Message  string          `json:"message"`
	Data     json.RawMessage `json:"data"`
	Metadata struct {
		Total int `json:"_total"`
	} `json:"metadata"`
}

// flexInt tolerates the mixed code/status shapes Quark returns: numbers,
// numeric strings, and the literal "OK" used by pan.quark.cn/account/info.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" {
		*f = 0
		return nil
	}
	if n, err := strconv.Atoi(text); err == nil {
		*f = flexInt(n)
		return nil
	}
	switch strings.ToLower(text) {
	case "ok", "true":
		*f = 0
	case "false":
		*f = 1
	default:
		*f = -1
	}
	return nil
}

type ShareFile struct {
	FID           string `json:"fid"`
	ShareFIDToken string `json:"share_fid_token"`
	FileName      string `json:"file_name"`
	Dir           bool   `json:"dir"`
	Size          int64  `json:"size"`
	UpdatedAt     int64  `json:"updated_at"`
	ObjCategory   string `json:"obj_category"`
	IncludeItems  int    `json:"include_items"`
	FileNameRe    string `json:"-"`
}

type PathFID struct {
	FilePath string `json:"file_path"`
	FID      string `json:"fid"`
}

type Stoken struct {
	Stoken string `json:"stoken"`
	Title  string `json:"title"`
}

type Detail struct {
	Title string
	List  []ShareFile
}

type ShareRef struct {
	PwdID    string
	Passcode string
	PdirFID  string
	Paths    []SharePath
}

type SharePath struct {
	FID  string `json:"fid"`
	Name string `json:"name"`
}

var (
	rePwdID    = regexp.MustCompile(`/s/([A-Za-z0-9]+)`)
	rePasscode = regexp.MustCompile(`pwd=([A-Za-z0-9]+)`)
	reSubPath  = regexp.MustCompile(`/(\w{32})-?([^/]+)?`)
)

// ExtractShareURL parses the share id, passcode and the optional
// #/list/share/<fid>-<name>/... navigation path.
func ExtractShareURL(raw string) (ShareRef, error) {
	id := rePwdID.FindStringSubmatch(raw)
	if len(id) < 2 {
		return ShareRef{}, fmt.Errorf("share url has no /s/<id>: %s", raw)
	}
	ref := ShareRef{PwdID: id[1], PdirFID: "0"}
	if pwd := rePasscode.FindStringSubmatch(raw); len(pwd) >= 2 {
		ref.Passcode = pwd[1]
	}
	for _, match := range reSubPath.FindAllStringSubmatch(raw, -1) {
		name, err := url.PathUnescape(match[2])
		if err != nil {
			name = match[2]
		}
		ref.Paths = append(ref.Paths, SharePath{
			FID:  match[1],
			Name: strings.ReplaceAll(name, "*101", "-"),
		})
	}
	if len(ref.Paths) > 0 {
		ref.PdirFID = ref.Paths[len(ref.Paths)-1].FID
	}
	return ref, nil
}

func parseMParam(cookie string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(cookie, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "kps", "sign", "vcode":
			out[kv[0]] = strings.ReplaceAll(kv[1], "%25", "%")
		}
	}
	if len(out) != 3 {
		return nil
	}
	return out
}

var mobileParams = map[string]string{
	"device_model": "M2011K2C",
	"entry":        "default_clouddrive",
	"_t_group":     "0%3A_s_vp%3A1",
	"dmn":          "Mi%2B11",
	"fr":           "android",
	"pf":           "3300",
	"bi":           "35937",
	"ve":           "7.4.5.680",
	"ss":           "411x875",
	"mi":           "M2011K2C",
	"nt":           "5",
	"nw":           "0",
	"kt":           "4",
	"pr":           "ucpro",
	"sv":           "release",
	"dt":           "phone",
	"data_from":    "ucapi",
	"app":          "clouddrive",
	"kkkk":         "1",
}

type APIError struct {
	Op      string
	Status  int
	Code    int
	Message string
	Body    string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Body
	}
	return fmt.Sprintf("%s: status=%d code=%d %s", e.Op, e.Status, e.Code, msg)
}

func apiErr(op string, env *envelope, raw []byte) error {
	return &APIError{Op: op, Status: int(env.Status), Code: int(env.Code), Message: env.Message, Body: snippet(raw)}
}

func snippet(raw []byte) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 240 {
		return s[:240] + "..."
	}
	return s
}

func (c *Client) request(method, rawURL string, params url.Values, body any) (*envelope, []byte, error) {
	env, raw, _, err := c.doJSON(method, rawURL, params, body)
	return env, raw, err
}

func (c *Client) doJSON(method, rawURL string, params url.Values, body any) (*envelope, []byte, []*http.Cookie, error) {
	if params == nil {
		params = url.Values{}
	}
	// Share endpoints switch to the mobile host and drop the cookie when
	// the cookie carries mobile credentials, mirroring the Python client.
	mobile := len(c.mparam) == 3 && strings.Contains(rawURL, "share") && strings.HasPrefix(rawURL, c.baseURL)
	if mobile {
		rawURL = strings.Replace(rawURL, c.baseURL, c.baseURLApp, 1)
		for key, value := range mobileParams {
			params.Set(key, value)
		}
		for key, value := range c.mparam {
			params.Set(key, value)
		}
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, nil, err
	}
	u.RawQuery = params.Encode()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, nil, nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, u.String(), reader)
	if err != nil {
		return nil, nil, nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("user-agent", userAgent)
	if !mobile {
		req.Header.Set("cookie", c.cookie)
	}

	if c.debug {
		fmt.Printf("[debug] %s %s mobile=%v\n", method, redactURL(u), mobile)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, nil, nil, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, raw, nil, &APIError{
			Op:     "decode",
			Status: resp.StatusCode,
			Body:   snippet(raw),
		}
	}
	if c.debug {
		fmt.Printf("[debug] status=%d code=%d msg=%s\n", env.Status, env.Code, env.Message)
	}
	if resp.StatusCode >= 400 {
		return &env, raw, resp.Cookies(), apiErr("http", &env, raw)
	}
	return &env, raw, resp.Cookies(), nil
}

func (c *Client) GetStoken(pwdID, passcode string) (Stoken, error) {
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/share/sharepage/token", params, map[string]string{
		"pwd_id":   pwdID,
		"passcode": passcode,
	})
	if err != nil {
		return Stoken{}, err
	}
	if env.Code != 0 {
		return Stoken{}, apiErr("share token", env, raw)
	}
	var out Stoken
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return Stoken{}, fmt.Errorf("share token decode: %w", err)
	}
	if out.Stoken == "" {
		return Stoken{}, fmt.Errorf("share token: empty stoken (%s)", env.Message)
	}
	return out, nil
}

func (c *Client) GetDetail(pwdID, stoken, pdirFID string, fetchShare, fetchFullPath int) (Detail, error) {
	var out Detail
	for page := 1; page <= 500; page++ {
		params := url.Values{
			"pr":                    {"ucpro"},
			"fr":                    {"pc"},
			"pwd_id":                {pwdID},
			"stoken":                {stoken},
			"pdir_fid":              {pdirFID},
			"force":                 {"0"},
			"_page":                 {strconv.Itoa(page)},
			"_size":                 {"50"},
			"_fetch_banner":         {"0"},
			"_fetch_share":          {strconv.Itoa(fetchShare)},
			"_fetch_total":          {"1"},
			"_sort":                 {"file_type:asc,updated_at:desc"},
			"ver":                   {"2"},
			"fetch_share_full_path": {strconv.Itoa(fetchFullPath)},
		}
		env, raw, err := c.request(http.MethodGet, c.baseURL+"/1/clouddrive/share/sharepage/detail", params, nil)
		if err != nil {
			return out, err
		}
		if env.Code != 0 {
			return out, apiErr("share detail", env, raw)
		}
		var data struct {
			Title string      `json:"title"`
			List  []ShareFile `json:"list"`
		}
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return out, fmt.Errorf("share detail decode: %w", err)
		}
		if out.Title == "" {
			out.Title = data.Title
		}
		if len(data.List) == 0 {
			break
		}
		out.List = append(out.List, data.List...)
		if env.Metadata.Total > 0 && len(out.List) >= env.Metadata.Total {
			break
		}
	}
	return out, nil
}

func (c *Client) GetFids(paths []string) ([]PathFID, error) {
	normalized := make([]string, len(paths))
	for i, path := range paths {
		normalized[i] = normalizePath(path)
	}
	paths = normalized

	var out []PathFID
	for len(paths) > 0 {
		n := min(50, len(paths))
		chunk := paths[:n]
		paths = paths[n:]

		params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}}
		env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/file/info/path_list", params, map[string]any{
			"file_path": chunk,
			"namespace": "0",
		})
		if err != nil {
			return nil, err
		}
		if env.Code != 0 {
			return nil, apiErr("path list", env, raw)
		}
		var data []PathFID
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, fmt.Errorf("path list decode: %w", err)
		}
		out = append(out, data...)
	}
	return out, nil
}

func (c *Client) Mkdir(dirPath string) (string, error) {
	dirPath = normalizePath(dirPath)
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}, "uc_param_str": {""}}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/file", params, map[string]any{
		"pdir_fid":      "0",
		"file_name":     "",
		"dir_path":      dirPath,
		"dir_init_lock": false,
	})
	if err != nil {
		return "", err
	}
	if env.Code != 0 {
		return "", apiErr("mkdir", env, raw)
	}
	var data struct {
		FID string `json:"fid"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", fmt.Errorf("mkdir decode: %w", err)
	}
	if data.FID == "" {
		return "", fmt.Errorf("mkdir: empty fid (%s)", env.Message)
	}
	return data.FID, nil
}

func (c *Client) SaveFile(fidList, fidTokenList []string, toPdirFID, pwdID, stoken string) (string, error) {
	params := url.Values{
		"pr":           {"ucpro"},
		"fr":           {"pc"},
		"uc_param_str": {""},
		"app":          {"clouddrive"},
		"__dt":         {newDT()},
		"__t":          {newT()},
	}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/share/sharepage/save", params, map[string]any{
		"fid_list":       fidList,
		"fid_token_list": fidTokenList,
		"to_pdir_fid":    toPdirFID,
		"pwd_id":         pwdID,
		"stoken":         stoken,
		"pdir_fid":       "0",
		"scene":          "link",
	})
	if err != nil {
		return "", err
	}
	if env.Code != 0 {
		return "", apiErr("save", env, raw)
	}
	var data struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", fmt.Errorf("save decode: %w", err)
	}
	if data.TaskID == "" {
		return "", fmt.Errorf("save: empty task_id (%s)", env.Message)
	}
	return data.TaskID, nil
}

type TaskResult struct {
	Status        int
	TaskTitle     string
	SaveAsTopFIDs []string
	UnarchiveList []ShareFile
}

// WaitTask polls a task until it finishes; unarchive results arrive in
// UnarchiveList, save results in SaveAsTopFIDs.
func (c *Client) WaitTask(taskID string, timeout time.Duration) (TaskResult, error) {
	deadline := time.Now().Add(timeout)
	for retry := 0; ; retry++ {
		params := url.Values{
			"pr":           {"ucpro"},
			"fr":           {"pc"},
			"uc_param_str": {""},
			"task_id":      {taskID},
			"retry_index":  {strconv.Itoa(retry)},
			"__dt":         {newDT()},
			"__t":          {newT()},
		}
		env, raw, err := c.request(http.MethodGet, c.baseURL+"/1/clouddrive/task", params, nil)
		if err != nil {
			return TaskResult{}, err
		}
		if env.Status != 0 && env.Status != 200 {
			return TaskResult{}, apiErr("task", env, raw)
		}
		if env.Code != 0 {
			return TaskResult{}, apiErr("task", env, raw)
		}
		var data struct {
			Status    int    `json:"status"`
			TaskTitle string `json:"task_title"`
			SaveAs    struct {
				TopFids []string `json:"save_as_top_fids"`
			} `json:"save_as"`
			Unarchive struct {
				List []ShareFile `json:"list"`
			} `json:"unarchive_result"`
		}
		if len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, &data); err != nil {
				return TaskResult{}, fmt.Errorf("task decode: %w", err)
			}
		}
		if data.Status == 2 {
			return TaskResult{
				Status:        data.Status,
				TaskTitle:     data.TaskTitle,
				SaveAsTopFIDs: data.SaveAs.TopFids,
				UnarchiveList: data.Unarchive.List,
			}, nil
		}
		if time.Now().After(deadline) {
			return TaskResult{}, fmt.Errorf("task %s timed out after %s", taskID, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (c *Client) QueryTask(taskID string) ([]string, error) {
	result, err := c.WaitTask(taskID, 3*time.Minute)
	if err != nil {
		return nil, err
	}
	return result.SaveAsTopFIDs, nil
}

func (c *Client) LsDir(pdirFID string) ([]ShareFile, error) {
	var out []ShareFile
	for page := 1; page <= 500; page++ {
		params := url.Values{
			"pr":                   {"ucpro"},
			"fr":                   {"pc"},
			"uc_param_str":         {""},
			"pdir_fid":             {pdirFID},
			"_page":                {strconv.Itoa(page)},
			"_size":                {"50"},
			"_fetch_total":         {"1"},
			"_fetch_sub_dirs":      {"0"},
			"_sort":                {"file_type:asc,updated_at:desc"},
			"_fetch_full_path":     {"0"},
			"fetch_all_file":       {"1"},
			"fetch_risk_file_name": {"1"},
		}
		env, raw, err := c.request(http.MethodGet, c.baseURL+"/1/clouddrive/file/sort", params, nil)
		if err != nil {
			return nil, err
		}
		if env.Code != 0 {
			return nil, apiErr("ls", env, raw)
		}
		var data struct {
			List []ShareFile `json:"list"`
		}
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return nil, fmt.Errorf("ls decode: %w", err)
		}
		if len(data.List) == 0 {
			break
		}
		out = append(out, data.List...)
		if env.Metadata.Total > 0 && len(out) >= env.Metadata.Total {
			break
		}
	}
	return out, nil
}

func (c *Client) Rename(fid, fileName string) error {
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}, "uc_param_str": {""}}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/file/rename", params, map[string]string{
		"fid":       fid,
		"file_name": fileName,
	})
	if err != nil {
		return err
	}
	if env.Code != 0 {
		return apiErr("rename", env, raw)
	}
	return nil
}

func (c *Client) CookieHeader() string { return c.cookie }
func (c *Client) UserAgent() string    { return userAgent }

type DownloadItem struct {
	DownloadURL string `json:"download_url"`
}

// Download resolves direct download URLs plus the cookie they need.
func (c *Client) Download(fids []string) ([]DownloadItem, string, error) {
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}, "uc_param_str": {""}}
	env, raw, cookies, err := c.doJSON(http.MethodPost, c.baseURL+"/1/clouddrive/file/download", params, map[string]any{
		"fids": fids,
	})
	if err != nil {
		return nil, "", err
	}
	if env.Code != 0 {
		return nil, "", apiErr("download", env, raw)
	}
	var items []DownloadItem
	if err := json.Unmarshal(env.Data, &items); err != nil {
		return nil, "", fmt.Errorf("download decode: %w", err)
	}
	var parts []string
	for _, cookie := range cookies {
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return items, strings.Join(parts, "; "), nil
}

func (c *Client) MoveFiles(fids []string, toPdirFID string) error {
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}, "uc_param_str": {""}}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/file/move", params, map[string]any{
		"filelist":     fids,
		"to_pdir_fid":  toPdirFID,
		"exclude_fids": []string{},
		"action_type":  1,
	})
	if err != nil {
		return err
	}
	if env.Code != 0 {
		return apiErr("move", env, raw)
	}
	return nil
}

func (c *Client) Unarchive(fid, toPdirFID string) (string, error) {
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}, "uc_param_str": {""}}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/archive/unarchive", params, map[string]any{
		"fid":           fid,
		"to_pdir_fid":   toPdirFID,
		"conflict_mode": 3,
		"suffix_type":   0,
		"pwd":           "",
		"select_mode":   0,
	})
	if err != nil {
		return "", err
	}
	if env.Code != 0 {
		return "", apiErr("unarchive", env, raw)
	}
	var data struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", fmt.Errorf("unarchive decode: %w", err)
	}
	if data.TaskID == "" {
		return "", fmt.Errorf("unarchive: empty task_id (%s)", env.Message)
	}
	return data.TaskID, nil
}

func (c *Client) Delete(fids []string) (string, error) {
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}, "uc_param_str": {""}}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/file/delete", params, map[string]any{
		"action_type":  2,
		"filelist":     fids,
		"exclude_fids": []string{},
	})
	if err != nil {
		return "", err
	}
	if env.Code != 0 {
		return "", apiErr("delete", env, raw)
	}
	var data struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", fmt.Errorf("delete decode: %w", err)
	}
	return data.TaskID, nil
}

type RecycleItem struct {
	RecordID string `json:"record_id"`
	FID      string `json:"fid"`
}

func (c *Client) RecycleList() ([]RecycleItem, error) {
	params := url.Values{
		"pr":           {"ucpro"},
		"fr":           {"pc"},
		"uc_param_str": {""},
		"_page":        {"1"},
		"_size":        {"30"},
	}
	env, raw, err := c.request(http.MethodGet, c.baseURL+"/1/clouddrive/file/recycle/list", params, nil)
	if err != nil {
		return nil, err
	}
	if env.Code != 0 {
		return nil, apiErr("recycle list", env, raw)
	}
	var data struct {
		List []RecycleItem `json:"list"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, fmt.Errorf("recycle list decode: %w", err)
	}
	return data.List, nil
}

func (c *Client) RecycleRemove(recordIDs []string) error {
	params := url.Values{"pr": {"ucpro"}, "fr": {"pc"}, "uc_param_str": {""}}
	env, raw, err := c.request(http.MethodPost, c.baseURL+"/1/clouddrive/file/recycle/remove", params, map[string]any{
		"select_mode": 2,
		"record_list": recordIDs,
	})
	if err != nil {
		return err
	}
	if env.Code != 0 {
		return apiErr("recycle remove", env, raw)
	}
	return nil
}

var reMultiSlash = regexp.MustCompile(`/{2,}`)

func normalizePath(path string) string {
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	return reMultiSlash.ReplaceAllString(path, "/")
}

func redactURL(u *url.URL) string {
	clone := *u
	query := clone.Query()
	for _, key := range []string{"stoken", "kps", "sign", "vcode"} {
		if query.Has(key) {
			query.Set(key, "***")
		}
	}
	clone.RawQuery = query.Encode()
	return clone.String()
}

func newDT() string {
	return strconv.Itoa(60000 + rand.Intn(240001))
}

func newT() string {
	return strconv.FormatFloat(float64(time.Now().UnixMilli())/1000, 'f', 6, 64)
}

// NormalizePath exposes the slash normalization the API requires.
func NormalizePath(path string) string { return normalizePath(path) }
