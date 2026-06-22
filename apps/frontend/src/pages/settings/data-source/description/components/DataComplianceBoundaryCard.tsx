const complianceItems = [
  "本应用仅用于研究展示，不提供交易功能与执行能力。",
  "不同数据提供方可能存在延迟差异，请以实际来源为准。",
  "涉及的凭据（如 API Key、Cookie、Proxy）仅本地加密存储，界面仅遮罩展示。",
  "所有数据请以官方公开披露与实际来源数据为准，使用前请自行判断与核实。",
];

export function DataComplianceBoundaryCard() {
  return (
    <section className="data-description-card data-description-small-card">
      <h2>D. 数据合规与边界</h2>
      <ul className="data-description-compliance-list">
        {complianceItems.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </section>
  );
}
