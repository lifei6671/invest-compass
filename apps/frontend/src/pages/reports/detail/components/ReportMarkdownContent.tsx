import { Empty } from "antd";
import type { ReactNode } from "react";
import { openExternalURL } from "../../../../services/coreClient";

type ReportMarkdownContentProps = {
  documentTitle?: string;
  markdown?: string;
};

type MarkdownBlock =
  | { kind: "heading"; level: 1 | 2 | 3; text: string; line: number }
  | { kind: "paragraph"; text: string; line: number }
  | { kind: "list"; items: string[]; line: number }
  | { kind: "code"; text: string; line: number }
  | { kind: "table"; headers: string[]; rows: string[][]; line: number };

export function ReportMarkdownContent({ documentTitle, markdown }: ReportMarkdownContentProps) {
  if (!markdown?.trim()) {
    return (
      <article className="report-detail-card report-markdown-content">
        <Empty description="暂无报告正文" />
      </article>
    );
  }

  const blocks = removeDuplicateDocumentTitle(parseMarkdownBlocks(markdown), documentTitle);

  return (
    <article className="report-detail-card report-markdown-content">
      {blocks.map((block) => renderMarkdownBlock(block))}
    </article>
  );
}

function removeDuplicateDocumentTitle(blocks: MarkdownBlock[], documentTitle?: string): MarkdownBlock[] {
  const first = blocks[0];
  if (!documentTitle?.trim() || !first || first.kind !== "heading" || first.level !== 1) {
    return blocks;
  }
  return plainMarkdownText(first.text) === documentTitle.trim() ? blocks.slice(1) : blocks;
}

function parseMarkdownBlocks(markdown: string): MarkdownBlock[] {
  const lines = markdown.split("\n");
  const blocks: MarkdownBlock[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index] ?? "";
    const trimmed = line.trim();
    const lineNumber = index + 1;

    if (!trimmed) {
      index += 1;
      continue;
    }

    const heading = trimmed.match(/^(#{1,3})\s+(.+)$/);
    if (heading) {
      blocks.push({ kind: "heading", level: heading[1].length as 1 | 2 | 3, text: heading[2].trim(), line: lineNumber });
      index += 1;
      continue;
    }

    if (trimmed.startsWith("```")) {
      const codeLines: string[] = [];
      index += 1;
      while (index < lines.length && !(lines[index] ?? "").trim().startsWith("```")) {
        codeLines.push(lines[index] ?? "");
        index += 1;
      }
      blocks.push({ kind: "code", text: codeLines.join("\n"), line: lineNumber });
      index += index < lines.length ? 1 : 0;
      continue;
    }

    if (isTableStart(lines, index)) {
      const headers = splitTableRow(lines[index]);
      index += 2;
      const rows: string[][] = [];
      while (index < lines.length && splitTableRow(lines[index]).length > 1) {
        rows.push(splitTableRow(lines[index]));
        index += 1;
      }
      blocks.push({ kind: "table", headers, rows, line: lineNumber });
      continue;
    }

    if (/^[-*]\s+/.test(trimmed)) {
      const items: string[] = [];
      while (index < lines.length && /^[-*]\s+/.test((lines[index] ?? "").trim())) {
        items.push((lines[index] ?? "").trim().replace(/^[-*]\s+/, ""));
        index += 1;
      }
      blocks.push({ kind: "list", items, line: lineNumber });
      continue;
    }

    const paragraphLines = [trimmed];
    index += 1;
    while (index < lines.length && shouldContinueParagraph(lines[index] ?? "")) {
      paragraphLines.push((lines[index] ?? "").trim());
      index += 1;
    }
    blocks.push({ kind: "paragraph", text: paragraphLines.join(" "), line: lineNumber });
  }

  return blocks;
}

function renderMarkdownBlock(block: MarkdownBlock): ReactNode {
  if (block.kind === "heading") {
    const id = block.level >= 2 ? `report-section-section-${block.line}` : undefined;
    if (block.level === 1) {
      return <h1 key={block.line}>{renderInlineMarkdown(block.text, `h-${block.line}`)}</h1>;
    }
    if (block.level === 2) {
      return <h2 key={block.line} id={id}>{renderInlineMarkdown(block.text, `h-${block.line}`)}</h2>;
    }
    return <h3 key={block.line} id={id}>{renderInlineMarkdown(block.text, `h-${block.line}`)}</h3>;
  }

  if (block.kind === "list") {
    return (
      <ul key={block.line}>
        {block.items.map((item, index) => (
          <li key={`${block.line}-${index}`}>{renderInlineMarkdown(item, `li-${block.line}-${index}`)}</li>
        ))}
      </ul>
    );
  }

  if (block.kind === "code") {
    return (
      <pre key={block.line} className="report-md-code">
        <code>{block.text}</code>
      </pre>
    );
  }

  if (block.kind === "table") {
    return (
      <table key={block.line} className="report-md-table">
        <thead>
          <tr>
            {block.headers.map((header, index) => <th key={`${block.line}-h-${index}`}>{renderInlineMarkdown(header, `th-${block.line}-${index}`)}</th>)}
          </tr>
        </thead>
        <tbody>
          {block.rows.map((row, rowIndex) => (
            <tr key={`${block.line}-r-${rowIndex}`}>
              {row.map((cell, cellIndex) => <td key={`${block.line}-r-${rowIndex}-${cellIndex}`}>{renderInlineMarkdown(cell, `td-${block.line}-${rowIndex}-${cellIndex}`)}</td>)}
            </tr>
          ))}
        </tbody>
      </table>
    );
  }

  return <p key={block.line}>{renderInlineMarkdown(block.text, `p-${block.line}`)}</p>;
}

function renderInlineMarkdown(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  const pattern = /(`[^`]+`|\*\*[^*]+\*\*|\[[^\]]+\]\([^)]+\))/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(text)) !== null) {
    if (match.index > lastIndex) {
      nodes.push(text.slice(lastIndex, match.index));
    }
    nodes.push(renderInlineToken(match[0], `${keyPrefix}-${match.index}`));
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    nodes.push(text.slice(lastIndex));
  }

  return nodes;
}

function plainMarkdownText(text: string): string {
  return text
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/[*_`]/g, "")
    .trim();
}

function renderInlineToken(token: string, key: string): ReactNode {
  if (token.startsWith("**") && token.endsWith("**")) {
    return <strong key={key}>{renderInlineMarkdown(token.slice(2, -2), `${key}-strong`)}</strong>;
  }
  if (token.startsWith("`") && token.endsWith("`")) {
    return <code key={key}>{token.slice(1, -1)}</code>;
  }
  const link = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
  if (link) {
    const href = safeMarkdownURL(link[2]);
    if (!href) {
      return <span key={key}>{link[1]}</span>;
    }
    return (
      <a
        key={key}
        href={href}
        onClick={(event) => {
          event.preventDefault();
          void openExternalURL(href).catch(() => undefined);
        }}
      >
        {renderInlineMarkdown(link[1], `${key}-link`)}
      </a>
    );
  }
  return token;
}

function shouldContinueParagraph(line: string): boolean {
  const trimmed = line.trim();
  return Boolean(trimmed) && !/^(#{1,3})\s+/.test(trimmed) && !/^[-*]\s+/.test(trimmed) && !trimmed.startsWith("```") && !isTableDivider(trimmed);
}

function isTableStart(lines: string[], index: number): boolean {
  const current = splitTableRow(lines[index]);
  const next = lines[index + 1]?.trim() ?? "";
  return current.length > 1 && isTableDivider(next);
}

function splitTableRow(line: string | undefined): string[] {
  const trimmed = line?.trim() ?? "";
  if (!trimmed.includes("|")) {
    return [];
  }
  return trimmed.replace(/^\|/, "").replace(/\|$/, "").split("|").map((cell) => cell.trim());
}

function isTableDivider(line: string): boolean {
  return /^\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?$/.test(line);
}

function safeMarkdownURL(value: string): string {
  const trimmed = value.trim();
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "https:" ? trimmed : "";
  } catch {
    return "";
  }
}
