package quark

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func accountServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(handler))
}

func TestAccountInfoAndSignFlow(t *testing.T) {
	var accountHits, growthHits, signHits int
	var signBody map[string]any
	server := accountServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/account/info":
			accountHits++
			if got := r.Header.Get("cookie"); got != "__pus=c; __uid=1; kps=k; sign=s; vcode=v" {
				t.Fatalf("account cookie = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": map[string]any{"nickname": "阿甘"},
			})
		case "/1/clouddrive/capacity/growth/info":
			growthHits++
			if got := r.URL.Query().Get("kps"); got != "k" {
				t.Fatalf("growth kps = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": map[string]any{
					"member_type":    "SUPER_VIP",
					"total_capacity": 6 << 40,
					"cap_composition": map[string]any{
						"sign_reward": 1024 * 1024 * 1024,
					},
					"cap_sign": map[string]any{
						"sign_daily":        false,
						"sign_daily_reward": 0,
						"sign_progress":     3,
						"sign_target":       7,
					},
				},
			})
		case "/1/clouddrive/capacity/growth/sign":
			signHits++
			if r.Method != http.MethodPost {
				t.Fatalf("sign method = %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&signBody); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": map[string]any{"sign_daily_reward": 50 * 1024 * 1024},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})
	defer server.Close()

	client := New("__pus=c; __uid=1; kps=k; sign=s; vcode=v")
	client.SetBaseURL(server.URL, server.URL)
	client.SetAccountURL(server.URL + "/account/info")

	info, err := client.GetAccountInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Nickname != "阿甘" {
		t.Fatalf("nickname = %q", info.Nickname)
	}

	growth, err := client.GetGrowthInfo()
	if err != nil {
		t.Fatal(err)
	}
	if growth.MemberType != "SUPER_VIP" || growth.CapSign.SignProgress != 3 || growth.CapSign.SignDaily {
		t.Fatalf("growth = %#v", growth)
	}
	if growth.CapComposition.SignReward != 1<<30 {
		t.Fatalf("sign reward = %d", growth.CapComposition.SignReward)
	}

	reward, err := client.SignGrowth()
	if err != nil {
		t.Fatal(err)
	}
	if reward != 50<<20 {
		t.Fatalf("reward = %d", reward)
	}
	if signBody["sign_cyclic"] != true {
		t.Fatalf("sign body = %#v", signBody)
	}
	if accountHits != 1 || growthHits != 1 || signHits != 1 {
		t.Fatalf("hits: account=%d growth=%d sign=%d", accountHits, growthHits, signHits)
	}
}

func TestAccountInfoWithoutUID(t *testing.T) {
	client := New("__pus=only")
	if client.HasUID() {
		t.Fatal("cookie without __uid should not report a web login")
	}
	if !New("__pus=a; __uid=1").HasUID() {
		t.Fatal("cookie with __uid should report a web login")
	}
}

func TestGrowthInfoErrorSurfacesMessage(t *testing.T) {
	server := accountServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 200, "code": 31001, "message": "登录失效",
		})
	})
	defer server.Close()

	client := New("kps=k; sign=s; vcode=v")
	client.SetBaseURL(server.URL, server.URL)
	if _, err := client.GetGrowthInfo(); err == nil {
		t.Fatal("expected growth info error")
	}
}

func TestSignGrowthErrorSurfacesCode(t *testing.T) {
	server := accountServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 200, "code": 31001, "message": "登录失效",
			"data": map[string]any{"sign_daily_reward": 0},
		})
	})
	defer server.Close()

	client := New("kps=k; sign=s; vcode=v")
	client.SetBaseURL(server.URL, server.URL)
	_, err := client.SignGrowth()
	if err == nil {
		t.Fatal("expected sign error")
	}
	var errAPI *APIError
	if !errors.As(err, &errAPI) || errAPI.Code != 31001 {
		t.Fatalf("err = %v", err)
	}
}

func TestAccountInfoAcceptsStringCode(t *testing.T) {
	server := accountServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"code":    "OK",
			"data":    map[string]any{"nickname": "大气的熊狸"},
		})
	})
	defer server.Close()

	client := New("__uid=1")
	client.SetAccountURL(server.URL)
	info, err := client.GetAccountInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Nickname != "大气的熊狸" {
		t.Fatalf("nickname = %q", info.Nickname)
	}
}
