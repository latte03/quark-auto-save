package mcp

func objectSchema(properties map[string]any, required ...string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerSchema(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

func booleanSchema(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func arraySchema(items map[string]any, description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       items,
		"description": description,
	}
}

func taskSchema(description string) map[string]any {
	schema := objectSchema(map[string]any{
		"taskname":                  stringSchema("任务名称，也用于 {TASKNAME} 变量"),
		"shareurl":                  stringSchema("夸克分享链接"),
		"savepath":                  stringSchema("保存目录，例如 /video/tv/剧名"),
		"pattern":                   stringSchema("正则匹配式，支持 $TV_MAGIC 等魔法键"),
		"replace":                   stringSchema("正则替换式，支持 {TASKNAME}/{SXX}/{E}/{EXT} 等变量"),
		"update_subdir":             stringSchema("追更时只更新指定子目录的匹配式"),
		"update_subdir_resave_mode": booleanSchema("追更子目录时重新转存"),
		"ignore_extension":          booleanSchema("比较已存在文件时忽略后缀"),
		"runweek": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "integer"},
			"description": "运行星期，1=周一 … 7=周日",
		},
		"enddate":  stringSchema("截止日期 YYYY-MM-DD，空为永久"),
		"startfid": stringSchema("起始文件 ID"),
		"addition": map[string]any{
			"type":                 "object",
			"description":          "插件配置（auto_unarchive/alist/aria2 等）",
			"additionalProperties": true,
		},
	})
	schema["description"] = description
	schema["additionalProperties"] = true
	return schema
}
