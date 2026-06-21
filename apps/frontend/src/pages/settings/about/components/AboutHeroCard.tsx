import appLogo from "../../../../assets/invest-compass-icon.png";

export function AboutHeroCard() {
  return (
    <section className="settings-basic-card settings-about-hero-card">
      <div className="settings-about-logo-frame">
        <img src={appLogo} alt="投研罗盘" />
      </div>
      <div className="settings-about-hero-copy">
        <div className="settings-about-title-row">
          <h1>投研罗盘&nbsp;&nbsp;Invest Compass</h1>
          <span>v0.1.0</span>
        </div>
        <h2>本地优先的 AI 投研桌面工作台</h2>
        <p>投研罗盘是一款面向个人投资者和研究者的 AI 投研助手，聚合行情、资讯与 AI 分析能力，帮助您更高效地进行投研研究与决策辅助。</p>
      </div>
      <div className="settings-about-illustration" aria-hidden="true">
        <div className="settings-about-chart-card settings-about-line-card">
          <svg viewBox="0 0 210 86" role="img">
            <path d="M10 66 L42 54 L72 58 L102 36 L132 44 L162 22 L196 34" fill="none" stroke="#1677ff" strokeWidth="3" />
            {[10, 42, 72, 102, 132, 162, 196].map((x, index) => (
              <circle key={x} cx={x} cy={[66, 54, 58, 36, 44, 22, 34][index]} r="4" fill="#ffffff" stroke="#1677ff" strokeWidth="2" />
            ))}
          </svg>
        </div>
        <div className="settings-about-chart-card settings-about-pie-card">
          <span className="settings-about-pie" />
          <i />
          <i />
        </div>
        <div className="settings-about-chart-card settings-about-list-card">
          <b />
          <b />
          <b />
          <b />
        </div>
        <div className="settings-about-chart-card settings-about-bar-card">
          <span />
          <span />
          <span />
          <span />
        </div>
      </div>
    </section>
  );
}
