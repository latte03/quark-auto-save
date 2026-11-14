import { useState } from "react";
import { EyeIcon, EyeOffIcon } from "lucide-react";
import { Button } from "../../components/ui/button";
import {
  CodeBlock,
  CodeBlockActions,
  CodeBlockContent,
  CodeBlockCopyButton,
  CodeBlockHeader,
  CodeBlockTitle,
} from "../../components/ui/code-block";
import { Field, FieldDescription, FieldLabel } from "../../components/ui/field";
import { Input } from "../../components/ui/input";
import { PageSection } from "../../components/PageSection";
import { useSettings } from "./SettingsContext";

// 生产环境控制台与 API 同源；开发模式下 Vite 只代理 /api，外部客户端要直连后端地址。
function mcpEndpoint() {
  if (typeof window === "undefined") return "/mcp";
  return `${window.location.origin}/mcp`;
}

export function AccessSection() {
  const { config } = useSettings();
  const [reveal, setReveal] = useState(false);
  const endpoint = mcpEndpoint();
  const snippet = JSON.stringify(
    {
      mcpServers: {
        qas: {
          url: endpoint,
          headers: { Authorization: `Bearer ${config.api_token}` },
        },
      },
    },
    null,
    2,
  );

  return (
    <PageSection
      id="settings-access"
      title="外部接入"
      description="把 MCP 端点与 API token 填进 Claude Code / Cursor 等客户端，即可用外部 Agent 管理任务。"
    >
      <div className="max-w-xl space-y-4">
        <Field>
          <FieldLabel htmlFor="mcp-endpoint">MCP 端点</FieldLabel>
          <div className="flex gap-2">
            <Input
              id="mcp-endpoint"
              readOnly
              value={endpoint}
              className="font-mono"
            />
            <CodeBlockCopyButton value={endpoint} variant="outline" />
          </div>
          <FieldDescription>
            Streamable HTTP 传输，协议 2025-06-18。开发模式下前端地址只代理
            /api，外部客户端应直连后端（默认 http://127.0.0.1:8710/mcp）。
          </FieldDescription>
        </Field>

        <Field>
          <FieldLabel htmlFor="api-token">API Token</FieldLabel>
          <div className="flex gap-2">
            <Input
              id="api-token"
              readOnly
              type={reveal ? "text" : "password"}
              value={config.api_token}
              className="font-mono"
            />
            <Button
              type="button"
              variant="outline"
              size="icon"
              aria-label={reveal ? "隐藏 Token" : "显示 Token"}
              onClick={() => setReveal((value) => !value)}
            >
              {reveal ? <EyeOffIcon /> : <EyeIcon />}
            </Button>
            <CodeBlockCopyButton value={config.api_token} variant="outline" />
          </div>
          <FieldDescription>
            由 WebUI 账号密码派生，修改密码后失效，需要重新配置外部客户端；权限与
            WebUI 登录相同，请勿外传。
          </FieldDescription>
        </Field>

        <Field>
          <FieldLabel>客户端配置示例</FieldLabel>
          <CodeBlock>
            <CodeBlockHeader>
              <CodeBlockTitle>mcp.json</CodeBlockTitle>
              <CodeBlockActions>
                <CodeBlockCopyButton value={snippet} />
              </CodeBlockActions>
            </CodeBlockHeader>
            <CodeBlockContent wrap>
              {snippet}
            </CodeBlockContent>
          </CodeBlock>
          <FieldDescription>
            也可以用命令行添加：<code>claude mcp add --transport http qas {"<端点>"} --header "Authorization: Bearer {"<token>"}"</code>
          </FieldDescription>
        </Field>
      </div>
    </PageSection>
  );
}
