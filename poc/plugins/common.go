package plugins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

type Deps struct {
	HTTP *http.Client
	Log  func(format string, args ...any)
}

func (d Deps) client() *http.Client {
	if d.HTTP != nil {
		return d.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (d Deps) logf(format string, args ...any) {
	if d.Log != nil {
		d.Log(format, args...)
	}
}

// alistResponse is the envelope every AList endpoint returns.
type alistResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type alistFile struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Sign  string `json:"sign"`
}

type alistStorage struct {
	Driver    string `json:"driver"`
	MountPath string `json:"mount_path"`
	Addition  string `json:"addition"`
}

type alistClient struct {
	base  string
	token string
	deps  Deps
}

func newAlistClient(base, token string, deps Deps) *alistClient {
	return &alistClient{base: strings.TrimRight(base, "/"), token: token, deps: deps}
}

func (a *alistClient) headers() map[string]string {
	return map[string]string{"Authorization": a.token}
}

func (a *alistClient) url(endpoint string) string { return a.base + endpoint }

func (a *alistClient) get(endpoint string, query url.Values, headers map[string]string) (int, []byte, error) {
	request, err := http.NewRequest(http.MethodGet, a.url(endpoint), nil)
	if err != nil {
		return 0, nil, err
	}
	request.URL.RawQuery = query.Encode()
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return a.send(request)
}

func (a *alistClient) postJSON(endpoint string, payload any, headers map[string]string) (int, []byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	request, err := http.NewRequest(http.MethodPost, a.url(endpoint), bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return a.send(request)
}

func (a *alistClient) postForm(endpoint string, form url.Values, headers map[string]string) (int, []byte, error) {
	request, err := http.NewRequest(http.MethodPost, a.url(endpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return a.send(request)
}

func (a *alistClient) send(request *http.Request) (int, []byte, error) {
	response, err := a.deps.client().Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return response.StatusCode, nil, err
	}
	return response.StatusCode, body, nil
}

func postJSONURL(client *http.Client, endpoint string, payload any) (int, []byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return response.StatusCode, nil, err
	}
	return response.StatusCode, body, nil
}

func decodeAlist(body []byte) (alistResponse, error) {
	var out alistResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("decode alist response: %w", err)
	}
	return out, nil
}

// fsList mirrors the /api/fs/list calls the plugins make.
func (a *alistClient) fsList(dirPath string, refresh bool) ([]alistFile, alistResponse, error) {
	status, body, err := a.postJSON("/api/fs/list", map[string]any{
		"path":     dirPath,
		"refresh":  refresh,
		"password": "",
		"page":     1,
		"per_page": 0,
	}, a.headers())
	if err != nil {
		return nil, alistResponse{}, err
	}
	if status != http.StatusOK {
		return nil, alistResponse{}, fmt.Errorf("alist fs/list status %d", status)
	}
	response, err := decodeAlist(body)
	if err != nil {
		return nil, response, err
	}
	var data struct {
		Content []alistFile `json:"content"`
	}
	if len(response.Data) > 0 {
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return nil, response, err
		}
	}
	return data.Content, response, nil
}

func (a *alistClient) getStorage(storageID string) (alistStorage, error) {
	status, body, err := a.get("/api/admin/storage/get", url.Values{"id": {storageID}}, a.headers())
	if err != nil {
		return alistStorage{}, err
	}
	if status != http.StatusOK {
		return alistStorage{}, fmt.Errorf("alist storage/get status %d", status)
	}
	response, err := decodeAlist(body)
	if err != nil {
		return alistStorage{}, err
	}
	if response.Code != 200 {
		return alistStorage{}, fmt.Errorf("alist storage %s: %s", storageID, response.Message)
	}
	var storage alistStorage
	if err := json.Unmarshal(response.Data, &storage); err != nil {
		return alistStorage{}, err
	}
	return storage, nil
}

var reStoragePath = regexp.MustCompile(`^(/[^:]*):(/[^:]*)$`)
var reNumericID = regexp.MustCompile(`^\d+$`)

// storageMount resolves the AList mount path and the Quark root folder it
// exposes, accepting either "/mount:/quark/path" or a numeric storage id.
func (a *alistClient) storageMount(storageID, label string) (mount, quarkRoot string, ok bool) {
	if match := reStoragePath.FindStringSubmatch(storageID); match != nil {
		mount, quarkRoot = match[1], match[2]
		if _, response, err := a.fsList(mount, false); err != nil || response.Code != 200 {
			message := response.Message
			if err != nil {
				message = err.Error()
			}
			a.deps.logf("%s: 获取挂载路径失败❌ %s", label, message)
			return "", "", false
		}
		return mount, quarkRoot, true
	}
	if reNumericID.MatchString(storageID) {
		storage, err := a.getStorage(storageID)
		if err != nil {
			a.deps.logf("%s: 存储%s连接失败❌ %v", label, storageID, err)
			return "", "", false
		}
		switch storage.Driver {
		case "Quark":
			var addition struct {
				Cookie       string `json:"cookie"`
				RootFolderID string `json:"root_folder_id"`
			}
			if err := json.Unmarshal([]byte(storage.Addition), &addition); err != nil {
				a.deps.logf("%s: 解析存储配置失败❌ %v", label, err)
				return "", "", false
			}
			root, err := quarkFullPath(addition.Cookie, addition.RootFolderID)
			if err != nil {
				a.deps.logf("%s: 获取Quark路径出错 %v", label, err)
				return "", "", false
			}
			return storage.MountPath, root, true
		case "QuarkTV":
			a.deps.logf("%s: [QuarkTV]驱动⚠️ storage_id请手动填入 /Alist挂载路径:/Quark目录路径", label)
		default:
			a.deps.logf("%s: 不支持[%s]驱动 ❌", label, storage.Driver)
		}
		return "", "", false
	}
	a.deps.logf("%s: storage_id[%s]格式错误❌", label, storageID)
	return "", "", false
}

// quarkFullPath asks the Quark file API for the folder path of a storage's
// root_folder_id; the storage addition carries its own cookie.
func quarkFullPath(cookie, pdirFID string) (string, error) {
	if pdirFID == "0" || pdirFID == "" {
		return "/", nil
	}
	query := url.Values{
		"pr":               {"ucpro"},
		"fr":               {"pc"},
		"uc_param_str":     {""},
		"pdir_fid":         {pdirFID},
		"_page":            {"1"},
		"_size":            {"50"},
		"_fetch_total":     {"1"},
		"_fetch_sub_dirs":  {"0"},
		"_sort":            {"file_type:asc,updated_at:desc"},
		"_fetch_full_path": {"1"},
	}
	endpoint := "https://drive-h.quark.cn/1/clouddrive/file/sort?" + query.Encode()
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("cookie", cookie)
	request.Header.Set("content-type", "application/json")
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", err
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			FullPath []struct {
				FileName string `json:"file_name"`
			} `json:"full_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Code != 0 {
		return "", fmt.Errorf("quark sort code %d", payload.Code)
	}
	var full string
	for _, item := range payload.Data.FullPath {
		full += "/" + item.FileName
	}
	return full, nil
}

// joinAlistPath mirrors os.path.normpath(os.path.join(mount, relative)).
func joinAlistPath(mount, relative string) string {
	return path.Clean(path.Join(mount, strings.TrimLeft(relative, "/")))
}

func splitExt(name string) (root, ext string) {
	dot := strings.LastIndexByte(name, '.')
	slash := strings.LastIndexByte(name, '/')
	if dot <= slash {
		return name, ""
	}
	return name[:dot], name[dot:]
}

func baseName(name string) string {
	if index := strings.LastIndexByte(name, '/'); index >= 0 {
		return name[index+1:]
	}
	return name
}

func looseBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func decodeTaskConfig(raw []byte, target any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}
