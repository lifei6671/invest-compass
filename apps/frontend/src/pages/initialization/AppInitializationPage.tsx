import { App as AntdApp, Button } from "antd";
import { DesktopOutlined, InfoCircleOutlined, SafetyCertificateOutlined, UnorderedListOutlined } from "@ant-design/icons";
import appIconUrl from "../../assets/invest-compass-icon.png";
import { CurrentInitializationTaskCard } from "./components/CurrentInitializationTaskCard";
import { InitializationProgress } from "./components/InitializationProgress";
import { InitializationStepList } from "./components/InitializationStepList";
import { mockInitializationState } from "./mock";
import type { InitializationState } from "./types";

type AppInitializationPageProps = {
  state?: InitializationState;
};

export function AppInitializationPage(props: AppInitializationPageProps) {
  const { message } = AntdApp.useApp();
  const state = props.state ?? mockInitializationState;

  return (
    <div className="flex min-h-full flex-col items-center justify-center gap-4 bg-[#f6f8fb]">
      <section className="w-full max-w-[1120px] rounded-2xl border border-[#e5eaf3] bg-white px-14 py-11 shadow-[0_12px_36px_rgba(15,23,42,0.08)]">
        <header className="flex flex-col items-center text-center">
          <div className="flex items-center gap-3">
            <img src={appIconUrl} alt="投研罗盘" className="h-[52px] w-[52px] rounded-[12px] object-cover" />
            <div className="text-left">
              <div className="text-[20px] font-bold leading-6 text-[#111827]">投研罗盘</div>
              <div className="text-[13px] font-medium leading-5 text-[#111827]">Invest Compass</div>
            </div>
          </div>
          <h1 className="mt-7 text-[28px] font-bold leading-9 text-[#111827]">正在初始化本地数据环境</h1>
          <p className="mt-2 text-[15px] leading-6 text-[#64748b]">正在启动 Sidecar、准备数据库、加载分词器并同步基础数据，请稍候</p>
        </header>

        <InitializationProgress progress={state.progress} />

        <div className="mt-6 grid grid-cols-2 gap-4">
          <InitializationStepList steps={state.steps} />
          <CurrentInitializationTaskCard detail={state.taskDetail} logs={state.logs} />
        </div>

        <div className="mt-5 flex h-11 items-center gap-3 rounded-lg border border-[#b7d3ff] bg-[#eff6ff] px-[18px] text-[13px] text-[#475569]">
          <InfoCircleOutlined className="text-[18px] text-[#1677ff]" />
          <span>首次启动或版本升级后，可能需要较长时间完成数据库准备与本地缓存初始化。</span>
        </div>

        <div className="mt-5 flex justify-center gap-5">
          <Button
            className="h-[38px] w-[210px] rounded-md border-[#1677ff] text-[14px] font-semibold text-[#1677ff]"
            icon={<UnorderedListOutlined />}
            onClick={() => message.info("初始化日志待接入")}
          >
            查看初始化日志
          </Button>
          <Button
            className="h-[38px] w-[230px] rounded-md border-[#d9e2f1] text-[14px] font-semibold text-[#64748b]"
            icon={<DesktopOutlined />}
            onClick={() => message.info("初始化将在后台继续，请等待完成")}
          >
            后台继续（仅最小化等待）
          </Button>
        </div>
      </section>

      <div className="flex items-center gap-2 text-[14px] text-[#64748b]">
        <SafetyCertificateOutlined />
        仅供研究，不构成投资建议
      </div>
    </div>
  );
}
