import { Button, Input, Select, Tag } from "antd";
import { CalendarOutlined, ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import type { NewsFilters } from "../types";

type NewsFilterCardProps = {
  filters: NewsFilters;
  hotKeywords: string[];
  onChange: (patch: Partial<NewsFilters>) => void;
  onHotKeywordClick: (keyword: string) => void;
  onRefresh: () => void;
  isRefreshing?: boolean;
};

export function NewsFilterCard(props: NewsFilterCardProps) {
  return (
    <section className="news-filter-card">
      <div className="news-filter-grid">
        <label className="news-filter-field news-filter-keyword">
          <span>关键词搜索</span>
          <Input
            allowClear
            value={props.filters.keyword}
            placeholder="输入关键词，支持标题/摘要"
            suffix={<SearchOutlined />}
            onChange={(event) => props.onChange({ keyword: event.target.value })}
          />
        </label>
        <label className="news-filter-field">
          <span>关联股票</span>
          <Select
            value={props.filters.stock}
            options={["全部股票", "生益科技", "沪电股份", "新易盛", "中际旭创", "贵州茅台"].map((value) => ({
              value,
              label: value,
            }))}
            onChange={(stock) => props.onChange({ stock })}
          />
        </label>
        <label className="news-filter-field">
          <span>信息来源</span>
          <Select
            value={props.filters.source}
            options={[
              "全部来源",
              "财联社电报",
              "新浪财经",
              "华尔街见闻-全球7x24",
              "TradingView-PANews",
              "东方财富研报",
              "东方财富行业研究",
              "东方财富公告",
              "证券时报",
              "芯榜",
              "界面新闻",
              "同花顺资讯",
              "上海证券报",
              "Wind 资讯",
              "第一财经",
              "证券日报",
            ].map((value) => ({ value, label: value }))}
            onChange={(source) => props.onChange({ source })}
          />
        </label>
        <label className="news-filter-field">
          <span>行业</span>
          <Select
            value={props.filters.industry}
            options={["全部行业", "AI算力", "光模块", "PCB", "半导体设备", "电力设备", "储能", "机器人"].map((value) => ({
              value,
              label: value,
            }))}
            onChange={(industry) => props.onChange({ industry })}
          />
        </label>
        <label className="news-filter-field">
          <span>时间范围</span>
          <Select
            value={props.filters.timeRange}
            suffixIcon={<CalendarOutlined />}
            options={["今天", "近 3 天", "近 7 天", "近 30 天"].map((value) => ({ value, label: value }))}
            onChange={(timeRange) => props.onChange({ timeRange })}
          />
        </label>
        <Button type="primary" className="news-refresh-button" icon={<ReloadOutlined />} loading={props.isRefreshing} onClick={props.onRefresh}>
          刷新资讯
        </Button>
      </div>
      <div className="news-hot-keywords">
        <span>热门搜索：</span>
        {props.hotKeywords.map((keyword) => (
          <Tag key={keyword} className="news-hot-keyword-tag" onClick={() => props.onHotKeywordClick(keyword)}>
            {keyword}
          </Tag>
        ))}
      </div>
    </section>
  );
}
