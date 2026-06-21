import { App as AntApp, Tag } from "antd";
import { useState } from "react";
import { hotTopics } from "../mock";

const hotTabs = [
  { key: "industry", label: "行业热点" },
  { key: "concept", label: "概念热点" },
  { key: "market", label: "市场新闻" },
  { key: "watch", label: "重点观察" },
] as const;

export function HotTopicsCard() {
  const { message } = AntApp.useApp();
  const [activeTab, setActiveTab] = useState("industry");

  return (
    <section className="dashboard-surface dashboard-hot-card">
      <div className="dashboard-card-title dashboard-hot-title">
        <h2>今日热点</h2>
      </div>
      <div className="dashboard-hot-tabs">
        {hotTabs.map((tab) => (
          <button
            key={tab.key}
            className={activeTab === tab.key ? "active" : ""}
            type="button"
            onClick={() => {
              if (tab.key !== "industry") {
                message.info("该热点分类待接入");
                return;
              }
              setActiveTab(tab.key);
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <div className="dashboard-topic-list">
        {hotTopics.map((topic) => (
          <div key={topic.rank} className="dashboard-topic-row">
            <span className={`dashboard-topic-rank rank-${topic.rank}`}>{topic.rank}</span>
            <span className="dashboard-topic-name">{topic.name}</span>
            <Tag className="dashboard-topic-change">{topic.changePercent}</Tag>
            <span className="dashboard-topic-summary">{topic.summary}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
