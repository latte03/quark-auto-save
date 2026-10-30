package main

import (
	"flag"
	"fmt"
	"strings"

	"qas-poc/quark"
)

func cmdProbe(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	common.bind(fs)
	share := fs.String("share", "", "分享链接")
	pdir := fs.String("pdir", "0", "分享内子目录 fid，默认根目录")
	fs.Parse(args)
	if *share == "" {
		fatalf("probe 需要 -share")
	}

	client := common.client()
	ref := mustShareRef(*share)
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		fatalf("获取 stoken 失败: %v", err)
	}
	fmt.Printf("分享 ID %s · passcode %q\n", ref.PwdID, ref.Passcode)
	fmt.Printf("stoken %s\n", mask(stoken.Stoken))

	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, *pdir, 1, 1)
	if err != nil {
		fatalf("读取分享详情失败: %v", err)
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	fmt.Printf("分享标题 %q · %d 项\n", title, len(detail.List))
	printFiles(detail.List, 10)
}

func cmdLs(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	common.bind(fs)
	path := fs.String("path", "/", "目录路径")
	fid := fs.String("fid", "", "直接指定目录 fid（优先于 -path）")
	fs.Parse(args)

	client := common.client()
	target := *fid
	if target == "" {
		if *path == "" || *path == "/" {
			target = "0"
		} else {
			fids, err := client.GetFids([]string{*path})
			if err != nil {
				fatalf("解析目录 fid 失败: %v", err)
			}
			if len(fids) == 0 {
				fatalf("目录不存在: %s", *path)
			}
			target = fids[0].FID
		}
	}
	list, err := client.LsDir(target)
	if err != nil {
		fatalf("列目录失败: %v", err)
	}
	fmt.Printf("目录 %s（fid %s）· %d 项\n", *path, target, len(list))
	printFiles(list, len(list))
}

func cmdSave(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("save", flag.ExitOnError)
	common.bind(fs)
	share := fs.String("share", "", "分享链接")
	dest := fs.String("dest", "", "保存目录，例如 /poc-verify")
	limit := fs.Int("limit", 1, "最多转存几个顶层条目")
	write := fs.Bool("write", false, "真正执行转存；缺省只演练")
	renameTest := fs.Bool("rename-test", true, "转存后改名并改回，验证 rename 接口")
	fs.Parse(args)
	if *share == "" || *dest == "" {
		fatalf("save 需要 -share 与 -dest")
	}
	if *limit < 1 {
		fatalf("-limit 至少为 1")
	}

	client := common.client()
	ref := mustShareRef(*share)
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		fatalf("获取 stoken 失败: %v", err)
	}
	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, "0", 0, 0)
	if err != nil {
		fatalf("读取分享失败: %v", err)
	}
	picked := pickFiles(detail.List, *limit)
	if len(picked) == 0 {
		fatalf("分享为空，没有可转存的内容")
	}

	destFID := ""
	if fids, err := client.GetFids([]string{*dest}); err == nil && len(fids) > 0 {
		destFID = fids[0].FID
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	fmt.Printf("分享 %q · 选中 %d 项 · 目标 %s", title, len(picked), *dest)
	if destFID == "" {
		fmt.Print("（目录不存在）")
	} else {
		fmt.Printf("（fid %s）", destFID)
	}
	fmt.Println()
	for _, f := range picked {
		kind, meta := "文件", humanSize(f.Size)
		if f.Dir {
			kind = "目录"
			if f.IncludeItems > 0 {
				meta = fmt.Sprintf("%d 项", f.IncludeItems)
			}
		}
		fmt.Printf("  [%s] %s · %s\n", kind, f.FileName, meta)
	}
	if !*write {
		fmt.Println("演练结束：加 -write 才会真正转存。")
		return
	}
	if destFID == "" {
		destFID, err = client.Mkdir(*dest)
		if err != nil {
			fatalf("创建目录失败: %v", err)
		}
		fmt.Printf("已创建目录 %s（fid %s）\n", *dest, destFID)
	}

	fids := make([]string, 0, len(picked))
	tokens := make([]string, 0, len(picked))
	names := make([]string, 0, len(picked))
	for _, f := range picked {
		fids = append(fids, f.FID)
		tokens = append(tokens, f.ShareFIDToken)
		names = append(names, f.FileName)
	}
	taskID, err := client.SaveFile(fids, tokens, destFID, ref.PwdID, stoken.Stoken)
	if err != nil {
		fatalf("提交转存失败: %v", err)
	}
	fmt.Printf("转存任务 %s，等待完成...\n", taskID)
	topFIDs, err := client.QueryTask(taskID)
	if err != nil {
		fatalf("等待转存完成失败: %v", err)
	}
	fmt.Printf("转存完成，顶层 fid %v\n", topFIDs)

	saved, err := client.LsDir(destFID)
	if err != nil {
		fatalf("读取目标目录失败: %v", err)
	}
	byFID := map[string]quark.ShareFile{}
	for _, f := range saved {
		byFID[f.FID] = f
	}
	if len(topFIDs) == 0 {
		for _, f := range saved {
			for _, name := range names {
				if f.FileName == name {
					topFIDs = append(topFIDs, f.FID)
				}
			}
		}
	}
	for _, fid := range topFIDs {
		name := byFID[fid].FileName
		if name == "" {
			name = "(未在目录列表中匹配到名字)"
		}
		fmt.Printf("  ✓ %s · %s\n", fid, name)
	}

	if *renameTest && len(topFIDs) > 0 {
		renameRoundTrip(client, topFIDs[0], byFID[topFIDs[0]].FileName, destFID)
	}
	fmt.Printf("测试文件保留在 %s，PoC 不会删除。\n", *dest)
}

func renameRoundTrip(client *quark.Client, fid, name, destFID string) {
	if name == "" || strings.HasPrefix(name, "poc-") {
		fmt.Println("rename 验证跳过：没有可用的文件名")
		return
	}
	renamed := "poc-" + name
	if err := client.Rename(fid, renamed); err != nil {
		fmt.Printf("  ✗ rename 失败: %v\n", err)
		return
	}
	if !nameExists(client, destFID, renamed) {
		fmt.Printf("  ✗ rename 返回成功，但目录中没找到 %q（可能仍在处理）\n", renamed)
	} else {
		fmt.Printf("  ✓ rename 成功：%s -> %s\n", name, renamed)
	}
	if err := client.Rename(fid, name); err != nil {
		fmt.Printf("  ✗ 改回原名失败，请手动改回：fid %s -> %q\n", fid, name)
		return
	}
	if nameExists(client, destFID, name) {
		fmt.Printf("  ✓ 已改回原名：%s\n", name)
	}
}

func nameExists(client *quark.Client, destFID, name string) bool {
	list, err := client.LsDir(destFID)
	if err != nil {
		return false
	}
	for _, f := range list {
		if f.FileName == name {
			return true
		}
	}
	return false
}

func cmdRename(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("rename", flag.ExitOnError)
	common.bind(fs)
	fid := fs.String("fid", "", "文件 fid")
	name := fs.String("name", "", "新文件名")
	write := fs.Bool("write", false, "真正执行重命名")
	fs.Parse(args)
	if *fid == "" || *name == "" {
		fatalf("rename 需要 -fid 与 -name")
	}
	if !*write {
		fmt.Printf("演练：将把 %s 重命名为 %q；加 -write 才执行。\n", *fid, *name)
		return
	}
	client := common.client()
	if err := client.Rename(*fid, *name); err != nil {
		fatalf("rename 失败: %v", err)
	}
	fmt.Printf("✓ %s -> %s\n", *fid, *name)
}

func pickFiles(list []quark.ShareFile, limit int) []quark.ShareFile {
	var picked []quark.ShareFile
	for _, f := range list {
		if !f.Dir {
			picked = append(picked, f)
			if len(picked) >= limit {
				return picked
			}
		}
	}
	for _, f := range list {
		if f.Dir {
			picked = append(picked, f)
			if len(picked) >= limit {
				break
			}
		}
	}
	return picked
}

func printFiles(list []quark.ShareFile, limit int) {
	if len(list) == 0 {
		fmt.Println("  (空目录)")
		return
	}
	for i, f := range list {
		if limit > 0 && i >= limit {
			fmt.Printf("  ... 还有 %d 项\n", len(list)-limit)
			return
		}
		kind := "文件"
		meta := humanSize(f.Size)
		if f.Dir {
			kind = "目录"
			meta = fmt.Sprintf("%d 项", f.IncludeItems)
		}
		fmt.Printf("  [%s] %s · %s\n", kind, f.FileName, meta)
	}
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 5 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func mustShareRef(raw string) quark.ShareRef {
	ref, err := quark.ExtractShareURL(raw)
	if err != nil {
		fatalf("%v", err)
	}
	return ref
}

func mask(s string) string {
	if len(s) <= 12 {
		return "***"
	}
	return s[:6] + "..." + s[len(s)-4:]
}
