import { App as AntApp, Button, Input, Modal, Table, Tag, type TableColumnsType } from "antd";
import { CloseOutlined, DownOutlined, InfoCircleOutlined, SearchOutlined } from "@ant-design/icons";
import { useState, type CSSProperties } from "react";
import { stockSearch, type StockSearchResult } from "../../services/coreClient";

type AddWatchlistModalProps = {
  open: boolean;
  onClose: () => void;
  onConfirm: (payload: { stock: StockSearchResult; tags: string[]; note: string }) => Promise<void>;
};

const defaultTags = ["核心标的", "长期跟踪", "消费"];

const marketTagStyle: Record<string, CSSProperties> = {
  A股: { backgroundColor: "#eaf3ff", color: "#1677ff", borderColor: "#cfe3ff" },
  港股: { backgroundColor: "#fff3e0", color: "#d97706", borderColor: "#ffe1ad" },
  美股: { backgroundColor: "#e8f8ef", color: "#16a34a", borderColor: "#c8edd8" },
};

export function AddWatchlistModal(props: AddWatchlistModalProps) {
  const { message } = AntApp.useApp();
  const [keyword, setKeyword] = useState("");
  const [selectedStock, setSelectedStock] = useState<StockSearchResult | null>(null);
  const [tags, setTags] = useState(defaultTags);
  const [tagInput, setTagInput] = useState("");
  const [note, setNote] = useState("");
  const [searchResults, setSearchResults] = useState<StockSearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const resetState = () => {
    setKeyword("");
    setSelectedStock(null);
    setTags(defaultTags);
    setTagInput("");
    setNote("");
    setSearchResults([]);
    setIsSearching(false);
    setIsSubmitting(false);
  };

  const closeModal = () => {
    props.onClose();
    resetState();
  };

  const confirmAdd = async () => {
    if (!selectedStock) {
      message.warning("请先选择要添加的股票");
      return;
    }
    try {
      setIsSubmitting(true);
      await props.onConfirm({ stock: selectedStock, tags, note });
      resetState();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "添加自选股失败");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSearch = async () => {
    const normalized = keyword.trim();
    if (!normalized) {
      setSearchResults([]);
      message.info("请输入股票名称、代码或拼音后搜索");
      return;
    }
    try {
      setIsSearching(true);
      const results = await stockSearch(normalized);
      setSearchResults(results);
      setSelectedStock(null);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "股票搜索失败");
    } finally {
      setIsSearching(false);
    }
  };

  const addTag = () => {
    const nextTag = tagInput.trim();
    if (!nextTag) {
      return;
    }
    if (!tags.includes(nextTag)) {
      setTags((current) => [...current, nextTag]);
    }
    setTagInput("");
  };

  const columns: TableColumnsType<StockSearchResult> = [
    {
      title: "股票名称",
      dataIndex: "name",
      width: 94,
      render: (value: string) => <span className="font-semibold text-[#1f2937]">{value}</span>,
    },
    {
      title: "标准代码",
      dataIndex: "symbol",
      width: 122,
      render: (value: string) => <span className="app-number text-[#1f2937]">{value}</span>,
    },
    {
      title: "市场",
      dataIndex: "market",
      width: 70,
      render: (value: string) => (
        <Tag className="m-0 rounded px-2 py-0 text-[12px] font-medium leading-5" style={marketTagStyle[value] ?? {}}>
          {value}
        </Tag>
      ),
    },
    { title: "交易所", dataIndex: "exchange", width: 116, ellipsis: true },
    {
      title: "操作",
      key: "action",
      align: "center",
      width: 76,
      render: (_, record) => {
        const selected = selectedStock?.symbol === record.symbol;
        return (
          <Button className="h-7 w-14 rounded-md border-[#1677ff] bg-white px-0 text-[13px] font-medium text-[#1677ff] hover:!bg-[#eaf3ff]" onClick={() => setSelectedStock(record)}>
            {selected ? "已选择" : "添加"}
          </Button>
        );
      },
    },
  ];

  return (
    <Modal
      aria-label="添加自选股"
      centered
      className="add-watchlist-modal"
      closable={false}
      destroyOnHidden
      footer={null}
      mask={{ closable: true }}
      open={props.open}
      styles={{
        mask: { backgroundColor: "rgba(15, 23, 42, 0.28)", backdropFilter: "blur(2px)" },
        body: { padding: 0 },
      }}
      width={650}
      onCancel={closeModal}
    >
      <div className="flex min-h-[630px] flex-col overflow-hidden rounded-xl border border-[#e5eaf3] bg-white shadow-[0_18px_48px_rgba(15,23,42,0.16)]">
        <header className="relative px-7 pt-[22px]">
          <h2 className="m-0 text-[18px] font-semibold leading-6 text-[#111827]">添加自选股</h2>
          <p className="m-0 mt-2 text-[13px] leading-5 text-[#6b7280]">搜索股票名称、代码或拼音，添加到你的关注列表。</p>
          <Button
            aria-label="关闭添加自选股弹窗"
            type="text"
            className="h-8 w-8 p-0 text-[#8a94a6] hover:!text-[#111827]"
            icon={<CloseOutlined className="text-[20px]" />}
            style={{ position: "absolute", right: 20, top: 20 }}
            onClick={closeModal}
          />
        </header>

        <main className="flex-1 px-7 pt-5">
          <Input
            allowClear
            className="h-9 rounded-md border-[#d9e2f1] text-[13px] shadow-none focus-within:border-[#1677ff] focus-within:shadow-[0_0_0_2px_rgba(22,119,255,0.12)]"
            placeholder="输入股票名称、代码或拼音，例如：茅台 / 600519 / maotai"
            prefix={<SearchOutlined className="mr-2 text-[#8a94a6]" />}
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onPressEnter={handleSearch}
          />

          <section className="mt-6">
            <h3 className="m-0 mb-2.5 text-[14px] font-semibold leading-5 text-[#1f2937]">搜索结果</h3>
            <Table
              rowKey="symbol"
              className="add-watchlist-result-table overflow-hidden rounded-lg border border-[#edf1f7]"
              columns={columns}
              dataSource={searchResults}
              loading={isSearching}
              pagination={false}
              size="small"
              tableLayout="fixed"
            />
          </section>

          <section className="mt-6">
            <h3 className="m-0 text-[14px] font-semibold leading-5 text-[#1f2937]">扩展信息（可选）</h3>
            <div className="mt-3 grid grid-cols-2 gap-4">
              <div>
                <label className="mb-2 block text-[13px] font-semibold text-[#374151]">标签</label>
                <Input
                  className="h-[34px] rounded-md border-[#d9e2f1] text-[13px]"
                  placeholder="输入或选择标签，回车确认"
                  suffix={<DownOutlined className="text-[#8a94a6]" onClick={() => message.info("标签候选待接入")} />}
                  value={tagInput}
                  onChange={(event) => setTagInput(event.target.value)}
                  onPressEnter={addTag}
                />
                <div className="mt-2 flex flex-wrap gap-2">
                  {tags.map((tag) => (
                    <Tag key={tag} closable className="m-0 rounded-[14px] border-[#e5eaf3] bg-[#f3f6fb] px-2.5 py-0 text-[12px] leading-6 text-[#374151]" onClose={() => setTags((current) => current.filter((value) => value !== tag))}>
                      {tag}
                    </Tag>
                  ))}
                </div>
              </div>
              <div>
                <label className="mb-2 block text-[13px] font-semibold text-[#374151]">备注</label>
                <Input.TextArea
                  className="h-[74px] resize-none rounded-md border-[#d9e2f1] text-[13px]"
                  maxLength={200}
                  placeholder="添加备注，记录你对该股票的关注点..."
                  value={note}
                  onChange={(event) => setNote(event.target.value.slice(0, 200))}
                />
                <div className="mt-1 text-right text-[12px] text-[#8a94a6]">{note.length}/200</div>
              </div>
            </div>
          </section>
        </main>

        <footer className="flex min-h-[72px] items-center justify-between gap-4 border-t border-[#edf1f7] px-7 py-4">
          <div className="flex min-w-0 items-center gap-2 text-[13px] leading-5 text-[#6b7280]">
            <InfoCircleOutlined className="shrink-0 text-[#8a94a6]" />
            <span className="truncate">部分数据可能存在延迟，请以公开披露信息和实际数据源为准。</span>
          </div>
          <div className="flex shrink-0 items-center gap-3">
            <Button className="h-8 w-16 rounded-md border-[#d9e2f1] bg-white px-0 text-[13px] text-[#374151]" onClick={closeModal}>
              取消
            </Button>
            <Button type="primary" loading={isSubmitting} className="h-8 w-[88px] rounded-md bg-[#1677ff] px-0 text-[13px] hover:!bg-[#4096ff]" onClick={confirmAdd}>
              确认添加
            </Button>
          </div>
        </footer>
      </div>
    </Modal>
  );
}
