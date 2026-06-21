import { SafetyCertificateOutlined } from "@ant-design/icons";
import { Button, Input } from "antd";

type ProxyBypassRulesCardProps = {
  value: string;
  onChange: (value: string) => void;
  onSave: () => void;
};

export function ProxyBypassRulesCard(props: ProxyBypassRulesCardProps) {
  return (
    <section className="settings-basic-card settings-proxy-side-card settings-proxy-bypass-card">
      <header className="settings-basic-card-header settings-proxy-card-header">
        <h2>绕过代理设置（可选）</h2>
        <p>配置不走代理的地址，多个地址用分号分隔</p>
      </header>
      <label className="settings-proxy-field-label">不走代理的地址</label>
      <Input.TextArea
        className="settings-proxy-bypass-textarea"
        value={props.value}
        placeholder="例如：localhost;127.0.0.1;*.local"
        onChange={(event) => props.onChange(event.target.value)}
      />
      <p className="settings-proxy-helper-text">支持域名、IP、CIDR、通配符（*.example.com）</p>
      <Button type="primary" className="settings-proxy-save-button" icon={<SafetyCertificateOutlined />} onClick={props.onSave}>
        保存绕过规则
      </Button>
    </section>
  );
}
