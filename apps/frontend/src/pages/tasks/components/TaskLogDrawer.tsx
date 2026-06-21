import { useMemo, useState } from "react";
import {
  Alert,
  App as AntApp,
  Button,
  Checkbox,
  Drawer,
  Grid,
  Input,
  Select,
  Tag,
} from "antd";
import {
  CloseOutlined,
  DownloadOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import { buildTaskLogSummary, rawLogJson, taskLogEvents, taskLogRecords } from "../taskLogMock";
import { taskStatusLabels } from "../types";
import type { TaskStatus } from "../types";
import type { TaskLogDrawerProps, TaskLogLevel, TaskLogTab } from "../taskLogTypes";
import { ExecutionLogTable } from "./ExecutionLogTable";
import { RawJsonPanel } from "./RawJsonPanel";
import { TaskLogSummaryCard } from "./TaskLogSummaryCard";
import { TaskLogTabs } from "./TaskLogTabs";

const statusColorMap: Record<TaskStatus, string> = {
  RUNNING: "blue",
  SUCCESS: "green",
  FAILED: "red",
  CANCELLED: "default",
};

const logLevelOptions: Array<"全部级别" | TaskLogLevel> = ["全部级别", "INFO", "WARN", "ERROR"];
const stageOptions = ["全部阶段", "quote_fetch", "kline_fetch", "calc_macd", "prompt_build", "stream_timeout", "stream_failed"];

export function TaskLogDrawer({ open, task, onClose }: TaskLogDrawerProps) {
  const { message } = AntApp.useApp();
  const screens = Grid.useBreakpoint();
  const [activeTab, setActiveTab] = useState<TaskLogTab>("logs");
  const [keyword, setKeyword] = useState("");
  const [level, setLevel] = useState<"全部级别" | TaskLogLevel>("全部级别");
  const [stage, setStage] = useState("全部阶段");
  const [errorsOnly, setErrorsOnly] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);

  const summary = task ? buildTaskLogSummary(task) : null;
  const drawerWidth = screens.xl ? "clamp(620px, 48vw, 760px)" : screens.md ? "72vw" : "100vw";
  const filteredLogs = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return taskLogRecords.filter((record) => {
      const keywordMatched =
        !normalizedKeyword ||
        record.message.toLowerCase().includes(normalizedKeyword) ||
        record.module.toLowerCase().includes(normalizedKeyword) ||
        record.stage.toLowerCase().includes(normalizedKeyword);
      const levelMatched = level === "全部级别" || record.level === level;
      const stageMatched = stage === "全部阶段" || record.stage === stage;
      const errorMatched = !errorsOnly || record.level === "ERROR";
      return keywordMatched && levelMatched && stageMatched && errorMatched;
    });
  }, [errorsOnly, keyword, level, stage]);

  const handleCopyCurrentLogs = async () => {
    const content = filteredLogs.map((record) => `${record.time} [${record.level}] ${record.module}/${record.stage} ${record.message}`).join("\n");
    await navigator.clipboard.writeText(content);
    message.success("当前日志已复制");
  };

  return (
    <Drawer
      className="task-log-drawer"
      rootClassName="task-log-drawer-root"
      placement="right"
      open={open && Boolean(task)}
      width={drawerWidth}
      mask={!screens.md}
      maskClosable
      keyboard
      closable={false}
      destroyOnHidden={false}
      onClose={onClose}
      styles={{ body: { padding: 0 } }}
    >
      {task && summary ? (
        <div className="task-log-drawer-content">
          <header className="task-log-drawer-header">
            <div>
              <h2>完整日志</h2>
              <p>任务执行详情与排障信息</p>
            </div>
            <div className="task-log-drawer-header-actions">
              <Tag color={statusColorMap[summary.status]}>{taskStatusLabels[summary.status]}</Tag>
              <button type="button" aria-label="关闭日志抽屉" onClick={onClose}>
                <CloseOutlined />
              </button>
            </div>
          </header>

          <TaskLogSummaryCard summary={summary} />
          <TaskLogTabs activeTab={activeTab} onChange={setActiveTab} />

          <div className="task-log-drawer-body">
            {activeTab === "events" ? <EventTimeline /> : null}
            {activeTab === "logs" ? (
              <ExecutionLogsTab
                failed={task.status === "FAILED"}
                keyword={keyword}
                level={level}
                stage={stage}
                errorsOnly={errorsOnly}
                autoScroll={autoScroll}
                filteredLogs={filteredLogs}
                onKeywordChange={setKeyword}
                onLevelChange={setLevel}
                onStageChange={setStage}
                onErrorsOnlyChange={setErrorsOnly}
                onAutoScrollChange={setAutoScroll}
              />
            ) : null}
            {activeTab === "diagnosis" ? <DiagnosisTab /> : null}
            {activeTab === "context" ? <ContextTab task={task} /> : null}
          </div>

          <footer className="task-log-footer">
            <Button onClick={handleCopyCurrentLogs}>复制当前日志</Button>
            <Button type="primary" icon={<DownloadOutlined />} onClick={() => message.info("导出脱敏日志待接入")}>
              导出脱敏日志
            </Button>
            <Button onClick={onClose}>关闭</Button>
          </footer>
        </div>
      ) : null}
    </Drawer>
  );
}

type ExecutionLogsTabProps = {
  failed: boolean;
  keyword: string;
  level: "全部级别" | TaskLogLevel;
  stage: string;
  errorsOnly: boolean;
  autoScroll: boolean;
  filteredLogs: typeof taskLogRecords;
  onKeywordChange: (value: string) => void;
  onLevelChange: (value: "全部级别" | TaskLogLevel) => void;
  onStageChange: (value: string) => void;
  onErrorsOnlyChange: (value: boolean) => void;
  onAutoScrollChange: (value: boolean) => void;
};

function ExecutionLogsTab(props: ExecutionLogsTabProps) {
  return (
    <>
      {props.failed ? (
        <Alert
          className="task-log-error-alert"
          type="error"
          showIcon
          message="错误摘要：模型服务响应超时，可检查代理、API Key 或更换模型后重试。"
        />
      ) : null}

      <div className="task-log-toolbar">
        <Input
          className="task-log-search"
          allowClear
          prefix={<SearchOutlined />}
          placeholder="搜索日志关键字"
          value={props.keyword}
          onChange={(event) => props.onKeywordChange(event.target.value)}
        />
        <Select
          className="task-log-level-select"
          value={props.level}
          options={logLevelOptions.map((value) => ({ label: value, value }))}
          onChange={props.onLevelChange}
        />
        <Select
          className="task-log-stage-select"
          value={props.stage}
          options={stageOptions.map((value) => ({ label: value, value }))}
          onChange={props.onStageChange}
        />
        <Checkbox checked={props.errorsOnly} onChange={(event) => props.onErrorsOnlyChange(event.target.checked)}>
          仅看错误
        </Checkbox>
        <Button
          className={props.autoScroll ? "task-log-scroll-button-active" : ""}
          icon={<PlayCircleOutlined />}
          onClick={() => props.onAutoScrollChange(true)}
        >
          自动滚动
        </Button>
        <Button icon={<PauseCircleOutlined />} onClick={() => props.onAutoScrollChange(false)}>
          暂停滚动
        </Button>
      </div>

      <ExecutionLogTable records={props.filteredLogs} />
      <div className="task-log-filter-count">当前显示 {props.filteredLogs.length} 条日志</div>
      <RawJsonPanel value={rawLogJson} />
    </>
  );
}

function EventTimeline() {
  return (
    <section className="task-log-event-panel">
      {taskLogEvents.map((event) => (
        <div className={`task-log-event-item task-log-event-${event.eventType.toLowerCase()}`} key={event.id}>
          <time>{event.time}</time>
          <span />
          <div>
            <strong>{event.eventType}</strong>
            <p>{event.description}</p>
          </div>
        </div>
      ))}
    </section>
  );
}

function DiagnosisTab() {
  const { message } = AntApp.useApp();

  return (
    <section className="task-log-diagnosis">
      <div>
        <h3>可能原因</h3>
        <ol>
          <li>模型服务响应超时</li>
          <li>代理配置异常或网络延迟过高</li>
          <li>API Key 权限、额度或模型不可用</li>
          <li>输出内容过长，超过模型响应时间</li>
        </ol>
      </div>
      <div>
        <h3>建议处理</h3>
        <ol>
          <li>检查代理设置并重新测试连接</li>
          <li>更换 AI 模型后重试</li>
          <li>降低最大输出 Token</li>
          <li>稍后重新发起分析任务</li>
        </ol>
      </div>
      <div className="task-log-diagnosis-actions">
        <Button type="primary" icon={<ReloadOutlined />} onClick={() => message.info("重新分析待接入")}>
          重新分析
        </Button>
        <Button icon={<SettingOutlined />} onClick={() => message.info("代理设置待接入")}>
          检查代理设置
        </Button>
      </div>
    </section>
  );
}

function ContextTab({ task }: { task: NonNullable<TaskLogDrawerProps["task"]> }) {
  const contextItems = [
    ["股票", task.stockCode && task.stockName ? `${task.stockName} ${task.stockCode}` : "—"],
    ["分析类型", task.title.includes("技术面") ? "技术面分析" : "个股综合分析"],
    ["使用模型", task.model ?? "DeepSeek-V3"],
    ["Prompt 模板", "默认个股分析模板"],
    ["行情数据", "已加载"],
    ["K线数据", "120 条"],
    ["技术指标", "MA / MACD / RSI / KDJ / BOLL"],
    ["新闻资讯", "36 条"],
    ["用户持仓", "未提供"],
    ["数据更新时间", "2025-05-20 15:30:00"],
  ];

  return (
    <section className="task-log-context">
      {contextItems.map(([label, value]) => (
        <div key={label}>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
      ))}
      <p>上下文摘要仅用于排障展示，已脱敏且不包含完整 Prompt、API Key 或用户隐私输入。</p>
    </section>
  );
}
