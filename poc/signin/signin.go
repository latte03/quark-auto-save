package signin

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"qas-poc/quark"
)

type Options struct {
	Out io.Writer
	// Collect receives login failures and newly signed messages for the
	// aggregated push; SignNotify=false keeps sign results local.
	Collect    func(string)
	SignNotify bool
}

type Result struct {
	Index    int
	Nickname string
	Active   bool
	NoUID    bool
	Already  bool
	Signed   bool
	Reward   int64
	Progress int
	Target   int
	Err      string
}

func (o Options) logf(format string, args ...any) {
	if o.Out != nil {
		fmt.Fprintf(o.Out, format+"\n", args...)
	}
}

func (o Options) collect(text string) {
	if o.Collect != nil {
		o.Collect(text)
	}
}

var vipLabels = map[string]string{
	"NORMAL":    "普通用户",
	"EXP_SVIP":  "88VIP",
	"SUPER_VIP": "SVIP",
	"Z_VIP":     "SVIP+",
}

// FormatBytes renders a byte count the same way the Python script does.
func FormatBytes(size int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	value := float64(size)
	index := 0
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	return fmt.Sprintf("%.2f %s", value, units[index])
}

// Run verifies every account and claims the daily capacity reward.
func Run(accounts []*quark.Client, opts Options) []Result {
	results := make([]Result, 0, len(accounts))
	for index, account := range accounts {
		results = append(results, runOne(account, index+1, opts))
	}
	return results
}

func runOne(account *quark.Client, index int, opts Options) Result {
	result := Result{Index: index}
	opts.logf("▶️ 验证第%d个账号", index)
	if !account.HasUID() {
		opts.logf("💡 不存在cookie必要参数，判断为仅签到")
		result.NoUID = true
	} else if info, err := account.GetAccountInfo(); err != nil {
		message := fmt.Sprintf("👤 第%d个账号登录失败，cookie无效❌", index)
		opts.logf("%s（%s）", message, errorText(err))
		opts.collect(message)
		result.Err = errorText(err)
	} else {
		result.Active = true
		result.Nickname = info.Nickname
		opts.logf("👤 账号昵称: %s✅", info.Nickname)
	}

	if !account.HasMobileParams() {
		opts.logf("⏭️ 移动端参数未设置，跳过签到")
		opts.logf("")
		return result
	}

	growth, err := account.GetGrowthInfo()
	if err != nil {
		opts.logf("⏭️ 签到进度读取异常，可能登录失效，跳过签到")
		opts.logf("")
		if result.Err == "" {
			result.Err = errorText(err)
		}
		return result
	}
	result.Progress = growth.CapSign.SignProgress
	result.Target = growth.CapSign.SignTarget

	vip := vipLabels[growth.MemberType]
	if vip == "" {
		vip = growth.MemberType
	}
	growthMessage := fmt.Sprintf("💾 %s 总空间：%s，签到累计获得：%s",
		vip, FormatBytes(growth.TotalCapacity), FormatBytes(growth.CapComposition.SignReward))

	if growth.CapSign.SignDaily {
		result.Already = true
		opts.logf("📅 签到记录: 今日已签到+%dMB，连签进度(%d/%d)✅\n%s",
			growth.CapSign.SignDailyReward/1024/1024, growth.CapSign.SignProgress, growth.CapSign.SignTarget, growthMessage)
		opts.logf("")
		return result
	}

	reward, err := account.SignGrowth()
	if err != nil {
		opts.logf("📅 签到异常: %s", errorText(err))
		opts.logf("")
		result.Err = errorText(err)
		return result
	}
	result.Signed = true
	result.Reward = reward
	message := fmt.Sprintf("📅 执行签到: 今日签到+%dMB，连签进度(%d/%d)✅\n%s",
		reward/1024/1024, growth.CapSign.SignProgress+1, growth.CapSign.SignTarget, growthMessage)
	opts.logf("%s", message)
	if opts.SignNotify {
		if result.Nickname != "" {
			message = strings.Replace(message, "今日", "["+result.Nickname+"]今日", 1)
		}
		opts.collect(message)
	}
	opts.logf("")
	return result
}

func errorText(err error) string {
	var apiErr *quark.APIError
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		return apiErr.Message
	}
	return err.Error()
}
