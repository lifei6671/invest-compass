import { Button, Input, Modal, Tag } from "antd";
import type { ReactNode } from "react";
import { useEffect, useState } from "react";

type StockTagsNoteCardProps = {
  tags: string[];
  note: string;
  editable?: boolean;
  saving?: boolean;
  onSave?: (value: { tags: string[]; note: string }) => Promise<void> | void;
};

export function StockTagsNoteCard(props: StockTagsNoteCardProps) {
  const [open, setOpen] = useState(false);
  const [draftTags, setDraftTags] = useState("");
  const [draftNote, setDraftNote] = useState("");
  const hasTags = props.tags.length > 0;
  const hasNote = props.note.trim().length > 0;

  useEffect(() => {
    if (open) {
      setDraftTags(props.tags.join("，"));
      setDraftNote(props.note);
    }
  }, [open, props.note, props.tags]);

  const save = async () => {
    if (!props.onSave) {
      return;
    }
    await props.onSave({ tags: parseTags(draftTags), note: draftNote.trim() });
    setOpen(false);
  };

  return (
    <section className="stock-detail-card px-5 py-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="m-0 text-[16px] font-semibold leading-6 text-[#111827]">我的标签与备注</h2>
        {props.editable ? (
          <Button type="link" className="h-7 px-0 text-[13px]" onClick={() => setOpen(true)}>
            编辑
          </Button>
        ) : (
          <span className="text-[12px] text-[#8a94a6]">加入自选后可编辑</span>
        )}
      </div>
      <div className="space-y-3 text-[13px]">
        <InfoRow
          label="标签"
          value={
            <div className="flex flex-wrap gap-2">
              {hasTags ? (
                props.tags.map((tag) => (
                  <Tag key={tag} className="m-0 rounded-md border-0 bg-[#f3f6fb] px-2.5 py-0.5 text-[12px] text-[#475569]">
                    {tag}
                  </Tag>
                ))
              ) : (
                <span className="text-[#8a94a6]">暂无</span>
              )}
            </div>
          }
        />
        <InfoRow label="备注" value={hasNote ? props.note : <span className="text-[#8a94a6]">暂无</span>} />
        <InfoRow label="来源" value={props.editable ? "自选股记录" : <span className="text-[#8a94a6]">未加入自选</span>} />
      </div>
      <Modal title="编辑标签与备注" open={open} confirmLoading={props.saving} okText="保存" cancelText="取消" onOk={() => void save()} onCancel={() => setOpen(false)} destroyOnHidden>
        <div className="space-y-4 pt-2">
          <div>
            <div className="mb-2 text-[13px] font-semibold text-[#374151]">标签</div>
            <Input value={draftTags} placeholder="多个标签用逗号、空格或顿号分隔" onChange={(event) => setDraftTags(event.target.value)} />
          </div>
          <div>
            <div className="mb-2 text-[13px] font-semibold text-[#374151]">备注</div>
            <Input.TextArea value={draftNote} rows={4} maxLength={200} showCount placeholder="记录你对该股票的关注点" onChange={(event) => setDraftNote(event.target.value)} />
          </div>
        </div>
      </Modal>
    </section>
  );
}

function parseTags(value: string) {
  return Array.from(new Set(value.split(/[\s,，、]+/).map((item) => item.trim()).filter(Boolean)));
}

function InfoRow(props: { label: string; value: ReactNode }) {
  return (
    <div className="grid grid-cols-[64px_1fr] gap-3">
      <span className="text-[#64748b]">{props.label}</span>
      <div className="text-[#475569]">{props.value}</div>
    </div>
  );
}
