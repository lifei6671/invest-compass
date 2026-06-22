import { ReloadOutlined } from "@ant-design/icons";
import { Button, Select } from "antd";
import type { CredentialTestResult, CredentialTestTarget } from "../types";
import { CredentialStatusTag } from "./CredentialStatusTag";

type CredentialConnectionTestCardProps = {
  targets: CredentialTestTarget[];
  target: string;
  result: CredentialTestResult;
  testing: boolean;
  onTargetChange: (value: string) => void;
  onRetest: () => void;
};

export function CredentialConnectionTestCard(props: CredentialConnectionTestCardProps) {
  return (
    <section className="credential-card credential-test-card">
      <h2>连接测试</h2>
      <label className="credential-side-label">测试目标</label>
      <Select className="credential-select credential-test-select" value={props.target} options={props.targets} onChange={props.onTargetChange} />
      <div className="credential-test-result">
        <div className="credential-result-heading">
          <strong>最近测试结果</strong>
          <CredentialStatusTag status={props.result.status} text={props.result.status === "success" ? "连接成功" : undefined} />
        </div>
        <KeyValue label="响应时间" value={props.result.responseTimeMs ? `${props.result.responseTimeMs} ms` : "—"} />
        <KeyValue label="最后测试时间" value={props.result.testedAt ?? "—"} />
        <div className="credential-result-messages">
          <span>测试说明</span>
          <ul>
            {props.result.messages.map((message) => (
              <li key={message}>{message}</li>
            ))}
          </ul>
        </div>
      </div>
      <Button className="credential-outline-button credential-retest-button" icon={<ReloadOutlined />} loading={props.testing} onClick={props.onRetest}>
        重新测试
      </Button>
    </section>
  );
}

function KeyValue(props: { label: string; value: string }) {
  return (
    <div className="credential-test-kv-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
