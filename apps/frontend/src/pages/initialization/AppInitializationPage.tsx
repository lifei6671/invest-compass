import { App as AntdApp, Button } from "antd";
import { DesktopOutlined, InfoCircleOutlined, SafetyCertificateOutlined, UnorderedListOutlined } from "@ant-design/icons";
import { useState } from "react";
import appIconUrl from "../../assets/invest-compass-icon.png";
import { logsOpenDirectory } from "../../services/coreClient";
import { CurrentInitializationTaskCard } from "./components/CurrentInitializationTaskCard";
import { InitializationProgress } from "./components/InitializationProgress";
import { InitializationStepList } from "./components/InitializationStepList";
import { initialBootPendingState } from "./initialState";
import type { InitializationState } from "./types";

type AppInitializationPageProps = {
  state?: InitializationState;
};

export function AppInitializationPage(props: AppInitializationPageProps) {
  const { message } = AntdApp.useApp();
  const state = props.state ?? initialBootPendingState;
  const [openingLogs, setOpeningLogs] = useState(false);

  const handleOpenLogs = async () => {
    setOpeningLogs(true);
    try {
      await logsOpenDirectory();
      message.success("已打开初始化日志目录");
    } catch (cause) {
      message.error(cause instanceof Error ? cause.message : "初始化日志目录打开失败");
    } finally {
      setOpeningLogs(false);
    }
  };

  return (
    <div className="h-screen w-full overflow-y-auto bg-white px-4 py-2 pb-5 text-[#111827]">
      <main className="mx-auto flex w-full max-w-[1180px] flex-col">
        <header className="flex flex-col items-center text-center">
          <div className="flex items-center gap-2.5">
            <img src={appIconUrl} alt="投研罗盘" className="h-9 w-9 rounded-[9px] object-cover shadow-[0_8px_24px_rgba(15,23,42,0.08)]" />
            <div className="text-left">
              <div className="text-[18px] font-bold leading-5 text-[#111827]">投研罗盘</div>
              <div className="text-[12px] font-semibold leading-4 text-[#111827]">Invest Compass</div>
            </div>
          </div>
          <h1 className="mt-3 text-[22px] font-bold leading-7 text-[#111827]">正在初始化本地数据环境</h1>
          <p className="mt-1 text-[13px] leading-5 text-[#64748b]">正在启动 Sidecar、准备数据库、加载分词器并同步基础数据，请稍候</p>
        </header>

        <InitializationProgress progress={state.progress} />

        <div className="mt-2.5 grid grid-cols-1 gap-2.5 lg:grid-cols-2">
          <InitializationStepList steps={state.steps} />
          <CurrentInitializationTaskCard detail={state.taskDetail} logs={state.logs} />
        </div>

        <div className="mt-2 flex min-h-7 items-center gap-2 rounded-lg border border-[#b7d3ff] bg-[#eff6ff] px-3 text-[12px] leading-5 text-[#475569]">
          <InfoCircleOutlined className="text-[14px] text-[#1677ff]" />
          <span>首次启动或版本升级后，可能需要较长时间完成数据库准备与本地缓存初始化。</span>
        </div>

        <div className="mt-2 flex flex-wrap justify-center gap-3">
          <Button
            className="h-[32px] w-[176px] rounded-md border-[#1677ff] text-[13px] font-semibold text-[#1677ff]"
            icon={<UnorderedListOutlined />}
            loading={openingLogs}
            onClick={handleOpenLogs}
          >
            查看初始化日志
          </Button>
          <Button
            className="h-[32px] w-[210px] rounded-md border-[#d9e2f1] text-[13px] font-semibold text-[#64748b]"
            icon={<DesktopOutlined />}
            onClick={() => message.info("初始化将在后台继续，请等待完成")}
          >
            后台继续（仅最小化等待）
          </Button>
        </div>
      </main>

      <div className="mt-1 flex items-center justify-center gap-2 text-[12px] text-[#64748b]">
        <SafetyCertificateOutlined />
        仅供研究，不构成投资建议
      </div>
    </div>
  );
}
