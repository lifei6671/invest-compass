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
          {props.result.status === "success" ? (
            <>
              <span className="settings-data-source-tag settings-data-source-tag-ok">成功</span>
              <p>响应时间：{props.result.responseTimeMs} ms</p>
              <p>检查时间：{props.result.checkedAt}</p>
            </>
          ) : props.result.status === "failed" ? (
            <>
              <span className="settings-data-source-tag settings-data-source-tag-error">失败</span>
              <p>请检查代理配置后重试</p>
            </>
          ) : (
            <>
              <span className="settings-data-source-tag settings-data-source-tag-muted">未测试</span>
              <p>真实代理连接测试待接入</p>
            </>
          )}
        </div>
      </div>
      <Button className="settings-basic-outline-button settings-proxy-test-button" icon={<PlayCircleOutlined />} loading={props.testing} onClick={props.onTest}>
        测试连接
      </Button>
    </section>
  );
}
