package signin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qas-poc/quark"
)

type fakeQuark struct {
	signed     bool
	progress   int
	target     int
	reward     int64
	growthErr  bool
	signCalls  int
	infoCalls  int
	memberType string
}

func (f *fakeQuark) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/account/info":
			f.infoCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": map[string]any{"nickname": "阿甘"},
			})
		case "/1/clouddrive/capacity/growth/info":
			if f.growthErr {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status": 200, "code": 31001, "message": "登录失效",
				})
				return
			}
			member := f.memberType
			if member == "" {
				member = "SUPER_VIP"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": map[string]any{
					"member_type":    member,
					"total_capacity": 6 << 40,
					"cap_composition": map[string]any{
						"sign_reward": 1 << 30,
					},
					"cap_sign": map[string]any{
						"sign_daily":        f.signed,
						"sign_daily_reward": 50 << 20,
						"sign_progress":     f.progress,
						"sign_target":       f.target,
					},
				},
			})
		case "/1/clouddrive/capacity/growth/sign":
			f.signCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": map[string]any{"sign_daily_reward": f.reward},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

func (f *fakeQuark) client(t *testing.T, cookie string) *quark.Client {
	t.Helper()
	server := f.server(t)
	t.Cleanup(server.Close)
	client := quark.New(cookie)
	client.SetBaseURL(server.URL, server.URL)
	client.SetAccountURL(server.URL + "/account/info")
	return client
}

func TestRunSignsNewDay(t *testing.T) {
	fake := &fakeQuark{progress: 3, target: 7, reward: 50 << 20}
	client := fake.client(t, "__pus=c; __uid=1; kps=k; sign=s; vcode=v")
	var out strings.Builder
	var collected []string
	results := Run([]*quark.Client{client}, Options{
		Out:        &out,
		SignNotify: true,
		Collect:    func(text string) { collected = append(collected, text) },
	})

	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	result := results[0]
	if !result.Active || !result.Signed || result.Already || result.Reward != 50<<20 {
		t.Fatalf("result = %#v", result)
	}
	if result.Nickname != "阿甘" || result.Progress != 3 || result.Target != 7 {
		t.Fatalf("result = %#v", result)
	}
	if fake.signCalls != 1 {
		t.Fatalf("sign calls = %d", fake.signCalls)
	}
	log := out.String()
	for _, want := range []string{"▶️ 验证第1个账号", "👤 账号昵称: 阿甘✅", "📅 执行签到: 今日签到+50MB，连签进度(4/7)✅", "💾 SVIP 总空间：6.00 TB"} {
		if !strings.Contains(log, want) {
			t.Fatalf("log missing %q:\n%s", want, log)
		}
	}
	if len(collected) != 1 || !strings.Contains(collected[0], "[阿甘]今日签到+50MB") {
		t.Fatalf("collected = %#v", collected)
	}
}

func TestRunAlreadySignedToday(t *testing.T) {
	fake := &fakeQuark{signed: true, progress: 4, target: 7, reward: 50 << 20}
	client := fake.client(t, "__uid=1; kps=k; sign=s; vcode=v")
	var collected []string
	results := Run([]*quark.Client{client}, Options{
		SignNotify: true,
		Collect:    func(text string) { collected = append(collected, text) },
	})
	if !results[0].Already || results[0].Signed {
		t.Fatalf("result = %#v", results[0])
	}
	if fake.signCalls != 0 {
		t.Fatalf("sign calls = %d", fake.signCalls)
	}
	if len(collected) != 0 {
		t.Fatalf("collected = %#v", collected)
	}
}

func TestRunSignNotifyDisabled(t *testing.T) {
	fake := &fakeQuark{progress: 1, target: 7, reward: 50 << 20}
	client := fake.client(t, "__uid=1; kps=k; sign=s; vcode=v")
	var collected []string
	results := Run([]*quark.Client{client}, Options{
		SignNotify: false,
		Collect:    func(text string) { collected = append(collected, text) },
	})
	if !results[0].Signed {
		t.Fatalf("result = %#v", results[0])
	}
	if len(collected) != 0 {
		t.Fatalf("collected = %#v", collected)
	}
}

func TestRunWithoutMobileParamsSkipsSign(t *testing.T) {
	fake := &fakeQuark{}
	client := fake.client(t, "__uid=1")
	var out strings.Builder
	results := Run([]*quark.Client{client}, Options{Out: &out, SignNotify: true})
	if !results[0].Active || results[0].Signed {
		t.Fatalf("result = %#v", results[0])
	}
	if !strings.Contains(out.String(), "移动端参数未设置") {
		t.Fatalf("log = %s", out.String())
	}
}

func TestRunWithoutUIDSkipsVerificationButSigns(t *testing.T) {
	fake := &fakeQuark{progress: 0, target: 7, reward: 10 << 20}
	client := fake.client(t, "kps=k; sign=s; vcode=v")
	var out strings.Builder
	results := Run([]*quark.Client{client}, Options{Out: &out, SignNotify: true})
	if !results[0].NoUID || results[0].Active {
		t.Fatalf("result = %#v", results[0])
	}
	if fake.infoCalls != 0 {
		t.Fatalf("account info calls = %d", fake.infoCalls)
	}
	if !results[0].Signed {
		t.Fatalf("result = %#v", results[0])
	}
	if !strings.Contains(out.String(), "判断为仅签到") {
		t.Fatalf("log = %s", out.String())
	}
}

func TestRunCollectsLoginFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/account/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": nil})
		case "/1/clouddrive/capacity/growth/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": nil})
		}
	}))
	defer server.Close()
	client := quark.New("__uid=1; kps=k; sign=s; vcode=v")
	client.SetBaseURL(server.URL, server.URL)
	client.SetAccountURL(server.URL + "/account/info")

	var collected []string
	results := Run([]*quark.Client{client}, Options{
		SignNotify: true,
		Collect:    func(text string) { collected = append(collected, text) },
	})
	if results[0].Err == "" || results[0].Active {
		t.Fatalf("result = %#v", results[0])
	}
	if len(collected) != 1 || !strings.Contains(collected[0], "cookie无效") {
		t.Fatalf("collected = %#v", collected)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:       "0.00 B",
		1023:    "1023.00 B",
		1024:    "1.00 KB",
		1536:    "1.50 KB",
		1 << 30: "1.00 GB",
		6 << 40: "6.00 TB",
	}
	for input, want := range cases {
		if got := FormatBytes(input); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", input, got, want)
		}
	}
}
