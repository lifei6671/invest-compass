import { DeleteOutlined, EditOutlined, ExperimentOutlined, StarFilled, StarOutlined } from "@ant-design/icons";
import { App as AntApp, Button, ConfigProvider, Pagination, Switch, Table, Tooltip } from "antd";
import { useEffect, useMemo, useState } from "react";
import type { ColumnsType } from "antd/es/table";
import { appAntdLocale } from "../../../../lib/antdLocale";
import { ModelStatusTag } from "./ModelStatusTag";
import type { ModelConfig } from "../types";

type ModelConfigTableProps = {
  providerName: string;
  configs: ModelConfig[];
  selectedConfigId: string | null;
  onCreate: () => void;
  onSelect: (config: ModelConfig) => void;
  onEdit: (config: ModelConfig) => void;
  onToggleStream: (id: string, enabled: boolean) => void;
  onSetDefault: (id: string) => void;
  onTest: (id: string) => void;
  onDelete: (id: string) => void;
};

export function ModelConfigTable(props: ModelConfigTableProps) {
  const { modal } = AntApp.useApp();
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  useEffect(() => {
    setCurrentPage(1);
  }, [props.providerName, props.configs.length]);

  const pagedConfigs = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return props.configs.slice(start, start + pageSize);
  }, [currentPage, pageSize, props.configs]);

  const columns: ColumnsType<ModelConfig> = [
    {
      title: "配置名称",
      dataIndex: "name",
      width: 92,
      render: (value: string, record) => <span className={record.isDefault ? "font-semibold text-[#1677ff]" : "text-[#1f2937]"}>{value}</span>,
    },
    {
      title: "Provider",
      dataIndex: "providerName",
      width: 96,
      render: (value: string) => <span className="leading-4 text-[#374151]">{value}</span>,
    },
    {
      title: "Base URL",
      dataIndex: "baseUrl",
      width: 138,
      render: (value: string) => (
        <Tooltip title={value}>
          <span className="block max-w-[128px] truncate text-[12px] text-[#374151]">{value}</span>
        </Tooltip>
      ),
    },
    {
      title: "模型名",
      dataIndex: "modelName",
      width: 104,
      render: (value: string) => <span className="block max-w-[96px] truncate text-[12px] text-[#374151]">{value}</span>,
    },
    {
      title: "流式输出",
      dataIndex: "streamEnabled",
      width: 72,
      align: "center",
      render: (value: boolean, record) => <Switch size="small" checked={value} onChange={(checked) => props.onToggleStream(record.id, checked)} />,
    },
    {
      title: "默认模型",
      dataIndex: "isDefault",
      width: 72,
      align: "center",
      render: (value: boolean, record) => (
        <Button
          aria-label={value ? `${record.name} 当前默认模型` : `设为默认模型 ${record.name}`}
          type="text"
          size="small"
          className="h-6 w-6 p-0"
          icon={value ? <StarFilled className="text-[#1677ff]" /> : <StarOutlined className="text-[#64748b]" />}
          onClick={(event) => {
            event.stopPropagation();
            if (!value) {
              props.onSetDefault(record.id);
            }
          }}
        />
      ),
    },
    {
      title: "Key 状态",
      dataIndex: "keyStatus",
      width: 92,
      render: (_, record) =>
        record.keyStatus.hasKey ? (
          <span className="block text-[12px] leading-4 text-[#374151]">
            已配置
            <br />
            <span className="app-number">{record.keyStatus.maskedKey}</span>
          </span>
        ) : (
          <span className="text-[12px] font-medium text-[#ff4d4f]">未配置</span>
        ),
    },
    {
      title: "连接状态",
      dataIndex: "connectionStatus",
      width: 86,
      render: (_, record) => <ModelStatusTag status={record.connectionStatus} />,
    },
    {
      title: "操作",
      width: 84,
      fixed: "right",
      render: (_, record) => (
        <div className="flex items-center gap-1">
          <Tooltip title="编辑配置" placement="top">
            <Button
              aria-label={`编辑 ${record.name}`}
              type="text"
              size="small"
              className="h-6 w-6 p-0 text-[#64748b]"
              icon={<EditOutlined />}
              onClick={(event) => {
                event.stopPropagation();
                props.onEdit(record);
              }}
            />
          </Tooltip>
          <Tooltip title="测试连接" placement="top">
            <Button
              aria-label={`测试连接 ${record.name}`}
              type="text"
              size="small"
              className="h-6 w-6 p-0 text-[#64748b]"
              icon={<ExperimentOutlined />}
              onClick={(event) => {
                event.stopPropagation();
                props.onTest(record.id);
              }}
            />
          </Tooltip>
          <Tooltip title="删除配置" placement="top">
            <Button
              aria-label={`删除 ${record.name}`}
              type="text"
              size="small"
              className="h-6 w-6 p-0 text-[#ff4d4f]"
              icon={<DeleteOutlined />}
              onClick={(event) => {
                event.stopPropagation();
                modal.confirm({
                  title: "确认删除配置？",
                  content: `删除后将从本地 mock 列表移除「${record.name}」。`,
                  okText: "删除",
                  okButtonProps: { danger: true },
                  cancelText: "取消",
                  onOk: () => props.onDelete(record.id),
                });
              }}
            />
          </Tooltip>
        </div>
      ),
    },
  ];

  return (
    <section className="settings-card flex min-h-0 flex-col p-4">
      <div className="mb-3 flex h-8 items-center justify-between">
        <h2 className="m-0 text-[15px] font-semibold text-[#111827]">
          模型配置列表 <span className="ml-1 text-[13px] font-normal text-[#64748b]">（{props.providerName}）</span>
        </h2>
        <Button type="primary" className="h-8 rounded-md bg-[#1677ff] px-4 text-[13px]" onClick={props.onCreate}>
          + 新建配置
        </Button>
      </div>
      <Table<ModelConfig>
        rowKey="id"
        size="small"
        className="model-config-table min-h-0 flex-1"
        columns={columns}
        dataSource={pagedConfigs}
        pagination={false}
        scroll={{ x: 880, y: 506 }}
        rowClassName={(record) => (record.id === props.selectedConfigId ? "model-config-selected-row" : "")}
        onRow={(record) => ({ onClick: () => props.onSelect(record) })}
      />
      <div className="mt-3 flex h-8 items-center justify-between">
        <ConfigProvider locale={appAntdLocale}>
          <Pagination
            className="model-config-pagination w-full"
            size="small"
            current={currentPage}
            total={props.configs.length}
            pageSize={pageSize}
            pageSizeOptions={[10, 20, 50]}
            showQuickJumper
            showSizeChanger
            showTotal={(total) => `共 ${total} 条`}
            onChange={(page, nextPageSize) => {
              setCurrentPage(page);
              setPageSize(nextPageSize);
            }}
          />
        </ConfigProvider>
      </div>
    </section>
  );
}
