import type { FAQItem } from "../types";

type DataFAQCardProps = {
  items: FAQItem[];
};

export function DataFAQCard({ items }: DataFAQCardProps) {
  return (
    <section className="data-description-card data-description-small-card">
      <h2>F. 常见问题</h2>
      <div className="data-description-faq-list">
        {items.map((item) => (
          <div key={item.id} className="data-description-faq-row">
            <span>{item.question}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
