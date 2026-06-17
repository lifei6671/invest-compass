import { ConfigProvider, Layout, Typography } from "antd";

const { Content } = Layout;

export function App() {
  return (
    <ConfigProvider>
      <Layout className="min-h-screen bg-slate-50">
        <Content className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-6 py-10">
          <Typography.Title level={1}>投研罗盘</Typography.Title>
          <Typography.Paragraph>
            工程骨架已启动。首版功能会按实施清单逐步接入真实 Rust command 和 Go core API。
          </Typography.Paragraph>
        </Content>
      </Layout>
    </ConfigProvider>
  );
}
