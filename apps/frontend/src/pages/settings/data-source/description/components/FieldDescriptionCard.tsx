import { Button } from "antd";
import type { FieldDescription } from "../types";

type FieldDescriptionCardProps = {
  items: FieldDescription[];
  onViewMore: () => void;
};

export function FieldDescriptionCard({ items, onViewMore }: FieldDescriptionCardProps) {
  const leftItems = items.slice(0, 4);
  const rightItems = items.slice(4);
  const rows = Array.from({ length: Math.max(leftItems.length, rightItems.length) }, (_, index) => ({
    left: leftItems[index],
    right: rightItems[index],
  }));

  return (
    <section className="data-description-card data-description-small-card">
      <h2>E. 字段说明</h2>
      <div className="data-description-field-table">
        {rows.map((row, index) => (
          <div key={row.left?.field ?? row.right?.field ?? index} className="data-description-field-row">
            <FieldCell item={row.left} />
            <FieldCell item={row.right} />
          </div>
        ))}
      </div>
      <Button className="data-description-link-button data-description-side-link" type="text" onClick={onViewMore}>
        查看更多字段说明 &gt;
      </Button>
    </section>
  );
}

function FieldCell({ item }: { item?: FieldDescription }) {
  if (!item) {
    return <span className="data-description-field-empty" />;
  }
  return (
    <span className="data-description-field-cell">
      <strong>{item.field}</strong>
      <span>{item.description}</span>
    </span>
  );
}
