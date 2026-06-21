import { PlayCircleOutlined } from "@ant-design/icons";
import { Button, Select } from "antd";
import { testTargetOptions, type ProxyTestResult, type ProxyTestTarget } from "../types";

type ProxyConnectionTestCardProps = {
  target: ProxyTestTarget;
  result: ProxyTestResult;
  testing: boolean;
  onTargetChange: (target: ProxyTestTarget) => void;
  onTest: () => void;
};

export function ProxyConnectionTestCard(props: ProxyConnectionTestCardProps) {
  return (
    <section className="settings-basic-card settings-proxy-side-card settings-proxy-test-card">
      <header className="settings-basic-card-header settings-proxy-card-header">
        <h2>连接测试</h2>
        <p>测试代理配置是否可正常访问外部网络</p>
      </header>
      <div className="settings-proxy-test-body">
        <label className="settings-proxy-field-label">测试目标</label>
        <Select<ProxyTestTarget>
          className="settings-basic-select settings-proxy-target-select"
          value={props.target}
          options={testTargetOptions}
          onChange={props.onTargetChange}
        />
        <div className="settings-proxy-test-result">
          <div className="settings-proxy-field-label">测试结果</div>
          <span className="settings-data-source-tag settings-data-source-tag-ok">成功</span>
          <p>响应时间：{props.result.responseTimeMs ?? 128} ms</p>
          <p>检查时间：{props.result.checkedAt ?? "2025-05-20 15:30:00"}</p>
        </div>
      </div>
      <Button className="settings-basic-outline-button settings-proxy-test-button" icon={<PlayCircleOutlined />} loading={props.testing} onClick={props.onTest}>
        测试连接
      </Button>
    </section>
  );
}
