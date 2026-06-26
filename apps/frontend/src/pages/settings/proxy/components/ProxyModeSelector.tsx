import { DisconnectOutlined, GlobalOutlined, HddOutlined } from "@ant-design/icons";
import { Radio } from "antd";
import type { ReactNode } from "react";
import { proxyModeOptions, type ProxyMode } from "../types";

type ProxyModeSelectorProps = {
  value: ProxyMode;
  onChange: (mode: ProxyMode) => void;
};

export function ProxyModeSelector(props: ProxyModeSelectorProps) {
  return (
    <div className="settings-proxy-mode-grid">
      {proxyModeOptions.map((item) => {
        const selected = props.value === item.mode;
        return (
          <button
            key={item.mode}
            type="button"
            aria-label={item.title}
            className={["settings-proxy-mode-option", selected ? "settings-proxy-mode-option-active" : ""].join(" ")}
            onClick={() => props.onChange(item.mode)}
          >
            <Radio checked={selected} />
            <div className="settings-proxy-mode-copy">
              <div className="settings-proxy-mode-title-row">
                <span>{item.title}</span>
                {selected && item.current ? <span className="settings-data-source-tag settings-data-source-tag-ok">当前使用</span> : null}
              </div>
              <p>{item.description}</p>
            </div>
            <span className="settings-proxy-mode-icon">{modeIcon(item.mode)}</span>
          </button>
        );
      })}
    </div>
  );
}

function modeIcon(mode: ProxyMode): ReactNode {
  if (mode === "custom") {
    return <HddOutlined />;
  }
  if (mode === "none") {
    return <DisconnectOutlined />;
  }
  return <GlobalOutlined />;
}
