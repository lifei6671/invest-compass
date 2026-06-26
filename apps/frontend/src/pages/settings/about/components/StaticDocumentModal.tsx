import { Button, Modal } from "antd";

export type StaticDocumentKey = "license" | "manual" | "release-notes";

type StaticDocumentModalProps = {
  documentKey: StaticDocumentKey | null;
  onClose: () => void;
};

type StaticDocument = {
  title: string;
  subtitle: string;
  sections: Array<{
    heading: string;
    paragraphs?: string[];
    items?: string[];
  }>;
};

const staticDocuments: Record<StaticDocumentKey, StaticDocument> = {
  license: {
    title: "内置 LICENSE",
    subtitle: "投研罗盘采用 GNU General Public License v3.0 发布。",
    sections: [
      {
        heading: "GNU General Public License v3.0",
        paragraphs: [
          "本软件遵循 GNU GPL v3.0。你可以在该许可证条款约束下复制、分发和修改本软件。",
          "如果你分发修改后的版本，应按照 GPL v3.0 要求保留相同的自由软件许可边界，并向接收者提供相应源代码。",
        ],
      },
      {
        heading: "无担保声明",
        paragraphs: [
          "本软件按现状提供，不附带任何明示或默示担保，包括但不限于适销性、特定用途适用性或非侵权担保。",
          "投研罗盘的行情、资讯、指标和 AI 输出仅用于研究辅助，不构成投资建议或回报保证。",
        ],
      },
      {
        heading: "第三方组件",
        paragraphs: [
          "应用中使用的第三方开源组件遵循各自许可证。正式发布包后续会补充更完整的第三方依赖许可证清单。",
        ],
      },
    ],
  },
  manual: {
    title: "内置用户手册",
    subtitle: "首版用户手册覆盖启动、配置、分析和排障的基本流程。",
    sections: [
      {
        heading: "首版使用流程",
        items: [
          "启动桌面应用，等待本地 Go Core 和 SQLite 初始化完成。",
          "在设置中心确认工作区、代理和数据源状态。",
          "配置 OpenAI-compatible 模型，并完成模型连通性测试。",
          "搜索股票、加入自选，查看行情、K 线、技术指标和资讯。",
          "选择内置或自定义 Prompt 模板，发起个股 AI 分析任务。",
          "在任务历史和报告历史中回看执行过程与分析结果。",
        ],
      },
      {
        heading: "风险边界",
        paragraphs: [
          "所有分析输出仅作研究辅助，不构成投资建议。",
          "应用不连接交易账户，不提供自动交易执行、真实资金交易、回报保证或持仓托管能力。",
          "一次性持仓输入只用于本次分析上下文，不作为持仓数据单独落库。",
        ],
      },
      {
        heading: "排障建议",
        items: [
          "数据源不可用时，先检查网络、代理模式和 Provider 凭据状态。",
          "AI 分析失败时，先确认模型配置、API Key 引用和 Prompt 模板是否完整。",
          "需要反馈问题时，请从关于页导出已二次脱敏的日志包。",
        ],
      },
    ],
  },
  "release-notes": {
    title: "内置发布说明",
    subtitle: "v0.1.0 内测版聚焦本地投研主链路和设置中心真实接线。",
    sections: [
      {
        heading: "v0.1.0 内测版",
        items: [
          "新增本地 Tauri 桌面壳、Go Core sidecar、SQLite 本地数据库和安全启动门禁。",
          "接入股票搜索、自选股、行情、K 线、技术指标和新闻资讯基础能力。",
          "接入 OpenAI-compatible 模型配置、Prompt 模板、AI 分析任务、任务历史和报告历史。",
          "补齐设置中心的工作区、代理、数据源、模型、Prompt、检查更新和日志导出主要链路。",
        ],
      },
      {
        heading: "已知边界",
        paragraphs: [
          "首版不包含自动下载、自动安装或静默升级能力。",
          "授权状态仅展示当前 FREE 状态，不提供激活码输入、购买或功能限制闭环。",
          "外部数据源受第三方服务可用性、频率限制和凭据配置影响，结果可能延迟或缺失。",
        ],
      },
      {
        heading: "合规提示",
        paragraphs: [
          "AI 报告必须保留数据时效、风险提示、事实/推断/观点区分和非投资建议声明。",
          "不得将任何输出理解为具体操作倾向、持有安排或价格点位建议。",
        ],
      },
    ],
  },
};

export function StaticDocumentModal(props: StaticDocumentModalProps) {
  const document = props.documentKey ? staticDocuments[props.documentKey] : null;
  return (
    <Modal
      className="settings-about-static-document-modal"
      destroyOnHidden
      footer={<Button aria-label="关闭" onClick={props.onClose}>关闭</Button>}
      open={Boolean(document)}
      title={document?.title}
      width={760}
      onCancel={props.onClose}
    >
      {document ? (
        <div className="settings-about-static-document-body">
          <p className="settings-about-static-document-subtitle">{document.subtitle}</p>
          {document.sections.map((section) => (
            <section key={section.heading} className="settings-about-static-document-section">
              <h3>{section.heading}</h3>
              {section.paragraphs?.map((paragraph) => <p key={paragraph}>{paragraph}</p>)}
              {section.items ? (
                <ul>
                  {section.items.map((item) => <li key={item}>{item}</li>)}
                </ul>
              ) : null}
            </section>
          ))}
        </div>
      ) : null}
    </Modal>
  );
}
