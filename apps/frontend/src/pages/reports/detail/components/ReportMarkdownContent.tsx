import { InfoCircleFilled } from "@ant-design/icons";

export function ReportMarkdownContent() {
  return (
    <article className="report-detail-card report-markdown-content">
      <section id="report-section-conclusion" className="report-md-section">
        <h2>1. 核心结论</h2>
        <div className="report-md-info-box">
          <InfoCircleFilled />
          <p>
            公司基本面稳健，覆铜板行业需求结构向高端化升级，短期关注成本端波动与订单变化，中期维持谨慎乐观，建议持续跟踪产品结构、毛利率及高端产能释放节奏。
          </p>
        </div>
      </section>

      <section id="report-section-quote" className="report-md-section">
        <h2>2. 当前行情状态</h2>
        <ul>
          <li>当前收盘价 25.68 元，较昨日上涨 +1.42%，日内震荡上行。</li>
          <li>成交额 8.92 亿元，换手率 1.72%，市场活跃度一般。</li>
          <li>近 20 日涨跌幅 +6.21%，表现强于沪深300。</li>
        </ul>
      </section>

      <section id="report-section-technical" className="report-md-section">
        <h2>3. 技术面观察</h2>
        <ul>
          <li>日K线位于20日均线之上，短期趋势偏多。</li>
          <li>MACD 金叉，动能柱为正，动能有所增强。</li>
          <li>RSI(12) 为 61.23，处于中高位，注意短期波动风险。</li>
          <li>均线系统：股价运行在 MA5 / MA10 上方，MA20 逐步走平。</li>
        </ul>
      </section>

      <section id="report-section-fundamental" className="report-md-section">
        <h2>4. 基本面观察</h2>
        <table className="report-md-table">
          <thead>
            <tr>
              <th>指标</th>
              <th>数值</th>
              <th>行业排名（约）</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>市盈率（TTM）</td>
              <td className="report-positive">24.18</td>
              <td>中等</td>
            </tr>
            <tr>
              <td>市净率（LF）</td>
              <td className="report-positive">3.32</td>
              <td>中等</td>
            </tr>
            <tr>
              <td>市销率（TTM）</td>
              <td className="report-positive">2.45</td>
              <td>中等</td>
            </tr>
            <tr>
              <td>净利率（TTM）</td>
              <td>11.83%</td>
              <td>中等偏上</td>
            </tr>
            <tr>
              <td>ROE（TTM）</td>
              <td>13.12%</td>
              <td>中等偏上</td>
            </tr>
          </tbody>
        </table>
        <p>公司盈利能力稳定，净利率和 ROE 处于行业中等偏上水平，现金流状况良好，资产结构健康，具备持续投入研发和产能扩张的能力。</p>
      </section>

      <section id="report-section-news" className="report-md-section">
        <h2>5. 消息面观察</h2>
        <ul>
          <li>一季度归母净利润同比增长 18.35%，产品结构持续优化。</li>
          <li>覆铜板需求回暖，高端产品订单相对饱满。</li>
          <li>公司持续增加研发投入，关注封装基板与高速材料进展。</li>
        </ul>
      </section>

      <section id="report-section-industry" className="report-md-section">
        <h2>6. 行业与竞争格局</h2>
        <p>覆铜板行业受电子终端需求、AI 服务器、通信设备与汽车电子共同影响。公司在客户结构、产品体系和研发投入方面具备一定优势，但仍需关注同行扩产和价格竞争。</p>
      </section>

      <section id="report-section-risks" className="report-md-section">
        <h2>7. 风险点</h2>
        <ul>
          <li>下游需求不及预期，行业竞争加剧。</li>
          <li>原材料价格波动导致成本上升。</li>
          <li>宏观经济波动及政策不确定性。</li>
        </ul>
      </section>

      <section id="report-section-tracking" className="report-md-section">
        <h2>8. 后续观察指标</h2>
        <ul>
          <li>订单及出货量变化情况。</li>
          <li>毛利率及费用率趋势。</li>
          <li>高端产品放量进度。</li>
        </ul>
      </section>

      <section id="report-section-notes" className="report-md-section">
        <h2>9. 说明</h2>
        <p>本报告基于本地 mock 行情、技术指标和资讯快照生成，仅用于界面展示与研究流程验证。</p>
      </section>
    </article>
  );
}
