import { App as AntApp, Button } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import Claude from "@lobehub/icons/es/Claude/components/Mono";
import DeepSeek from "@lobehub/icons/es/DeepSeek/components/Mono";
import Doubao from "@lobehub/icons/es/Doubao/components/Mono";
import Gemini from "@lobehub/icons/es/Gemini/components/Mono";
import LlmApi from "@lobehub/icons/es/LlmApi/components/Mono";
import LmStudio from "@lobehub/icons/es/LmStudio/components/Mono";
import Ollama from "@lobehub/icons/es/Ollama/components/Mono";
import OpenAI from "@lobehub/icons/es/OpenAI/components/Mono";
import Qwen from "@lobehub/icons/es/Qwen/components/Mono";
import { providers } from "../mock";
import type { AIProviderType, ProviderIconKey } from "../types";
import type { ReactNode } from "react";

type ProviderListPanelProps = {
  selectedProvider: AIProviderType;
  onSelect: (provider: AIProviderType) => void;
};

const providerIconMap: Record<ProviderIconKey, ReactNode> = {
  openai: <OpenAI size={18} />,
  deepseek: <DeepSeek size={18} />,
  qwen: <Qwen size={18} />,
  doubao: <Doubao size={18} />,
  gemini: <Gemini size={18} />,
  claude: <Claude size={18} />,
  ollama: <Ollama size={18} />,
  "lm-studio": <LmStudio size={18} />,
  "llm-api": <LlmApi size={18} />,
};

export function ProviderListPanel(props: ProviderListPanelProps) {
  const { message } = AntApp.useApp();
  return (
    <section className="settings-card flex min-h-0 flex-col p-3">
      <div className="mb-3 flex h-8 items-center justify-between px-1">
        <h2 className="m-0 text-[15px] font-semibold text-[#111827]">Provider 列表</h2>
        <Button aria-label="新增 Provider" size="small" className="h-7 w-7 rounded-md p-0 text-[#1677ff]" icon={<PlusOutlined />} onClick={() => message.info("新增 Provider 待接入")} />
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {providers.map((item) => {
          const active = item.id === props.selectedProvider;
          return (
            <Button
              key={item.id}
              type="text"
              block
              className={[
                "provider-list-item mb-1 flex h-[46px] w-full items-center justify-start gap-3 rounded-lg border-0 px-3 text-left text-[13px] transition",
                active ? "provider-list-item-active font-semibold text-[#1677ff]" : "bg-white text-[#374151] hover:bg-[#f8fbff]",
              ].join(" ")}
              onClick={() => props.onSelect(item.id)}
            >
              <span className="flex h-6 w-6 shrink-0 items-center justify-center text-current" aria-hidden="true">
                {providerIconMap[item.icon]}
              </span>
              <span className="min-w-0 whitespace-nowrap">{item.name}</span>
            </Button>
          );
        })}
      </div>
    </section>
  );
}
