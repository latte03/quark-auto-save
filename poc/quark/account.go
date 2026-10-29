package quark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const accountInfoURL = "https://pan.quark.cn/account/info"

type AccountInfo struct {
	Nickname string `json:"nickname"`
}

// GrowthInfo mirrors the capacity/growth payload used by the daily sign-in.
type GrowthInfo struct {
	MemberType     string `json:"member_type"`
	TotalCapacity  int64  `json:"total_capacity"`
	CapComposition struct {
		SignReward int64 `json:"sign_reward"`
	} `json:"cap_composition"`
	CapSign struct {
		SignDaily       bool  `json:"sign_daily"`
		SignDailyReward int64 `json:"sign_daily_reward"`
		SignProgress    int   `json:"sign_progress"`
		SignTarget      int   `json:"sign_target"`
	} `json:"cap_sign"`
}

// HasUID reports whether the cookie carries a web login (__uid). Cookies
// without it are treated as mobile-only credentials for signing.
func (c *Client) HasUID() bool {
	return strings.Contains(c.cookie, "__uid")
}

func (c *Client) SetAccountURL(raw string) {
	if raw != "" {
		c.accountURL = raw
	}
}

func (c *Client) GetAccountInfo() (AccountInfo, error) {
	params := url.Values{"fr": {"pc"}, "platform": {"pc"}}
	env, raw, err := c.request(http.MethodGet, c.accountURL, params, nil)
	if err != nil {
		return AccountInfo{}, err
	}
	var out AccountInfo
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			return AccountInfo{}, fmt.Errorf("account info decode: %w", err)
		}
	}
	if out.Nickname == "" {
		if env.Code != 0 && env.Message != "" {
			return AccountInfo{}, apiErr("account info", env, raw)
		}
		return AccountInfo{}, fmt.Errorf("账号信息为空")
	}
	return out, nil
}

func (c *Client) growthParams() url.Values {
	params := url.Values{"pr": {"ucpro"}, "fr": {"android"}}
	for _, key := range []string{"kps", "sign", "vcode"} {
		if value := c.mparam[key]; value != "" {
			params.Set(key, value)
		}
	}
	return params
}

func (c *Client) GetGrowthInfo() (GrowthInfo, error) {
	env, raw, err := c.request(http.MethodGet, c.baseURLApp+"/1/clouddrive/capacity/growth/info", c.growthParams(), nil)
	if err != nil {
		return GrowthInfo{}, err
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		if env.Message != "" {
			return GrowthInfo{}, apiErr("growth info", env, raw)
		}
		return GrowthInfo{}, fmt.Errorf("签到进度为空")
	}
	var out GrowthInfo
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return GrowthInfo{}, fmt.Errorf("growth info decode: %w", err)
	}
	return out, nil
}

// SignGrowth claims the daily capacity reward and returns the reward in bytes.
func (c *Client) SignGrowth() (int64, error) {
	env, raw, err := c.request(http.MethodPost, c.baseURLApp+"/1/clouddrive/capacity/growth/sign", c.growthParams(), map[string]any{
		"sign_cyclic": true,
	})
	if err != nil {
		return 0, err
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return 0, apiErr("growth sign", env, raw)
	}
	var out struct {
		SignDailyReward int64 `json:"sign_daily_reward"`
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return 0, fmt.Errorf("growth sign decode: %w", err)
	}
	return out.SignDailyReward, nil
}
