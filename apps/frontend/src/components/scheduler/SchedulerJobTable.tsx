import { DeleteOutlined, PauseCircleOutlined, PlayCircleOutlined, PoweroffOutlined } from "@ant-design/icons";
import { Button, Tag } from "antd";
import type { ProviderStatusItem } from "../../services/coreClient";
import type { SchedulerJob } from "../../services/scheduler";

type SchedulerJobTableProps = {
  jobs: SchedulerJob[];
  providers: ProviderStatusItem[];
  actionPending: string | null;
  onRunNow: (job: SchedulerJob) => void;
  onEdit: (job: SchedulerJob) => void;
  onSetEnabled: (job: SchedulerJob, enabled: boolean) => void;
  onDelete: (job: SchedulerJob) => void;
};

/// SchedulerJobTable 只展示和分发桌面调度任务操作，真实写入仍由上层页面调用固定 command。
export function SchedulerJobTable(props: SchedulerJobTableProps) {
  if (props.jobs.length === 0) {
    return <div className="px-4 py-8 text-sm text-slate-500">暂无调度任务</div>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full table-fixed text-left text-sm">
        <thead className="bg-slate-50 text-slate-500">
          <tr>
            <th className="w-[16%] px-4 py-2 font-medium">名称</th>
            <th className="w-[16%] px-4 py-2 font-medium">类型</th>
            <th className="w-[13%] px-4 py-2 font-medium">Cron</th>
            <th className="w-[13%] px-4 py-2 font-medium">下次运行</th>
            <th className="w-[8%] px-4 py-2 font-medium">状态</th>
            <th className="w-[10%] px-4 py-2 font-medium">最近结果</th>
            <th className="w-[12%] px-4 py-2 font-medium">最近错误</th>
            <th className="w-[12%] px-4 py-2 font-medium">操作</th>
          </tr>
        </thead>
        <tbody>
          {props.jobs.map((job) => {
            const unavailableReason = providerUnavailableReason(job, props.providers);
            return (
              <tr key={job.id} className="border-t border-slate-100">
                <td className="truncate px-4 py-3">{job.name || `任务 ${job.id}`}</td>
                <td className="truncate px-4 py-3">
                  <div className="flex flex-col gap-1">
                    <span>{job.cron_type}</span>
                    {unavailableReason ? <span className="truncate text-xs text-red-500">{unavailableReason}</span> : null}
                  </div>
                </td>
                <td className="truncate px-4 py-3">{job.cron_expr || "-"}</td>
                <td className="truncate px-4 py-3 font-mono text-xs">{nextSchedulerRunLabel(job)}</td>
                <td className="px-4 py-3">
                  <Tag color={job.enabled ? "green" : "default"}>{job.enabled ? "启用" : "停用"}</Tag>
                </td>
                <td className="px-4 py-3">{job.last_status ? <Tag>{job.last_status}</Tag> : "-"}</td>
                <td className="truncate px-4 py-3 text-xs text-slate-500">{job.last_error || "-"}</td>
                <td className="flex flex-wrap gap-2 px-4 py-3">
                  <Button
                    size="small"
                    aria-label="立即执行"
                    icon={<PlayCircleOutlined aria-hidden="true" />}
                    loading={props.actionPending === `run-${job.id}`}
                    disabled={Boolean(unavailableReason)}
                    onClick={() => props.onRunNow(job)}
                  >
                    立即执行
                  </Button>
                  <Button size="small" aria-label="编辑" onClick={() => props.onEdit(job)}>
                    编辑
                  </Button>
                  <Button
                    size="small"
                    aria-label={job.enabled ? "停用任务" : "启用任务"}
                    icon={job.enabled ? <PauseCircleOutlined aria-hidden="true" /> : <PoweroffOutlined aria-hidden="true" />}
                    loading={props.actionPending === `enabled-${job.id}`}
                    onClick={() => props.onSetEnabled(job, !job.enabled)}
                  >
                    {job.enabled ? "停用" : "启用"}
                  </Button>
                  <Button
                    size="small"
                    danger
                    aria-label="删除任务"
                    icon={<DeleteOutlined aria-hidden="true" />}
                    loading={props.actionPending === `delete-${job.id}`}
                    onClick={() => props.onDelete(job)}
                  >
                    删除
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function providerUnavailableReason(job: SchedulerJob, providers: ProviderStatusItem[]) {
  const providerName = providerNameForSchedulerJob(job);
  if (!providerName) {
    return "";
  }
  const provider = providers.find((item) => item.name === providerName);
  if (!provider || provider.available) {
    return "";
  }
  return provider.last_error || `${providerName} unavailable`;
}

function nextSchedulerRunLabel(job: SchedulerJob, now = new Date()) {
  if (!job.enabled || !job.cron_expr) {
    return "-";
  }
  const nextRun = nextRunFromFiveFieldCron(job.cron_expr, now);
  return nextRun ? formatDateTimeMinute(nextRun) : "-";
}

function nextRunFromFiveFieldCron(cronExpr: string, now: Date) {
  const fields = cronExpr.trim().split(/\s+/);
  if (fields.length !== 5) {
    return null;
  }
  const minutes = cronFieldEntries(fields[0], 0, 59);
  const hours = cronFieldEntries(fields[1], 0, 23);
  if (!minutes || !hours) {
    return null;
  }
  const base = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  for (let dayOffset = 0; dayOffset <= 366; dayOffset++) {
    const date = new Date(base);
    date.setDate(base.getDate() + dayOffset);
    if (!cronDateMatches(fields, date)) {
      continue;
    }
    for (const hour of hours) {
      for (const minute of minutes) {
        const candidate = new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour, minute, 0, 0);
        if (candidate > now) {
          return candidate;
        }
      }
    }
  }
  return null;
}

function cronDateMatches(fields: string[], date: Date) {
  return cronFieldMatches(fields[2], date.getDate(), 1, 31)
    && cronFieldMatches(fields[3], date.getMonth() + 1, 1, 12)
    && cronWeekdayMatches(fields[4], date.getDay());
}

function cronWeekdayMatches(field: string, weekday: number) {
  const entries = cronFieldEntries(field, 0, 7);
  if (!entries) {
    return false;
  }
  return entries.some((entry) => entry === weekday || (entry === 7 && weekday === 0));
}

function cronFieldMatches(field: string, value: number, min: number, max: number) {
  const entries = cronFieldEntries(field, min, max);
  return Boolean(entries?.includes(value));
}

function cronFieldEntries(field: string, min: number, max: number) {
  const values = new Set<number>();
  for (const segment of field.split(",")) {
    const entries = cronSegmentEntries(segment.trim(), min, max);
    if (!entries) {
      return null;
    }
    entries.forEach((value) => values.add(value));
  }
  return Array.from(values).sort((left, right) => left - right);
}

function cronSegmentEntries(segment: string, min: number, max: number) {
  if (!segment) {
    return null;
  }
  const [rangePart, stepPart] = segment.split("/");
  const step = stepPart ? Number(stepPart) : 1;
  if (!Number.isInteger(step) || step <= 0) {
    return null;
  }
  const range = cronSegmentRange(rangePart, min, max);
  if (!range) {
    return null;
  }
  const values: number[] = [];
  for (let value = range.start; value <= range.end; value += step) {
    values.push(value);
  }
  return values;
}

function cronSegmentRange(value: string, min: number, max: number) {
  if (value === "*") {
    return { start: min, end: max };
  }
  if (value.includes("-")) {
    const [start, end] = value.split("-").map((item) => Number(item));
    if (!Number.isInteger(start) || !Number.isInteger(end) || start < min || end > max || start > end) {
      return null;
    }
    return { start, end };
  }
  const single = Number(value);
  if (!Number.isInteger(single) || single < min || single > max) {
    return null;
  }
  return { start: single, end: single };
}

function formatDateTimeMinute(value: Date) {
  return `${value.getFullYear()}-${padDatePart(value.getMonth() + 1)}-${padDatePart(value.getDate())} ${padDatePart(value.getHours())}:${padDatePart(value.getMinutes())}`;
}

function padDatePart(value: number) {
  return String(value).padStart(2, "0");
}

function providerNameForSchedulerJob(job: SchedulerJob) {
  switch (job.cron_type) {
    case "cn_a_share_quote_refresh":
    case "cn_a_share_kline_refresh":
      return "market-provider";
    case "market_news_refresh":
    case "symbol_news_refresh":
      return "news-provider";
    default:
      return "";
  }
}
