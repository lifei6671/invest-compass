import { RightOutlined } from "@ant-design/icons";
import { Button } from "antd";
import type { FAQItem } from "../types";

type DataFAQCardProps = {
  items: FAQItem[];
  onOpenFAQ: () => void;
  onViewMore: () => void;
};

export function DataFAQCard({ items, onOpenFAQ, onViewMore }: DataFAQCardProps) {
  return (
    <section className="data-description-card data-description-small-card">
      <h2>F. 常见问题</h2>
      <div className="data-description-faq-list">
        {items.map((item) => (
          <button key={item.id} className="data-description-faq-row" type="button" onClick={onOpenFAQ}>
            <span>{item.question}</span>
            <RightOutlined />
          </button>
        ))}
      </div>
      <Button className="data-description-link-button data-description-side-link" type="text" onClick={onViewMore}>
        查看更多 FAQ &gt;
      </Button>
    </section>
  );
}
