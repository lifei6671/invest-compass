import { Component, type ErrorInfo, type ReactNode, useEffect, useState } from "react";
import { HashRouter, Link, Route, Routes } from "react-router-dom";
import { Alert, ConfigProvider, Layout, Spin, Typography } from "antd";
import { coreHealth, type CoreHealth } from "../services/coreClient";

const { Content, Header } = Layout;

type AppErrorBoundaryProps = {
  children: ReactNode;
};

type AppErrorBoundaryState = {
  hasError: boolean;
};

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(_error: Error, _info: ErrorInfo) {
    // 渲染异常只进入本地边界，不把错误对象或可能包含的上下文透出到页面。
  }

  render() {
    if (this.state.hasError) {
      return <Alert message="界面渲染失败" type="error" showIcon />;
    }

    return this.props.children;
  }
}

export function App() {
  return (
    <ConfigProvider>
      <AppErrorBoundary>
        <HashRouter>
          <Layout className="min-h-screen bg-slate-50">
            <Header className="flex items-center justify-between bg-white px-6 shadow-sm">
              <Typography.Title level={4} className="m-0">
                投研罗盘
              </Typography.Title>
              <nav aria-label="主导航" className="flex gap-4">
                <Link to="/">概览</Link>
              </nav>
            </Header>
            <Content className="mx-auto flex w-full max-w-5xl flex-col gap-4 px-6 py-10">
              <Routes>
                <Route path="/" element={<HomeRoute />} />
                <Route path="*" element={<Alert message="页面不存在" type="warning" showIcon />} />
              </Routes>
            </Content>
          </Layout>
        </HashRouter>
      </AppErrorBoundary>
    </ConfigProvider>
  );
}

function HomeRoute() {
  const [health, setHealth] = useState<CoreHealth | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;

    coreHealth()
      .then((value) => {
        if (active) {
          setHealth(value);
          setError(null);
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setError(cause instanceof Error ? cause.message : "本地核心服务连接失败");
        }
      });

    return () => {
      active = false;
    };
  }, []);

  return health ? (
    <Alert
      message="本地核心服务已连接"
      description={`版本 ${health.version}`}
      type="success"
      showIcon
    />
  ) : error ? (
    <Alert message="本地核心服务连接失败" description={error} type="error" showIcon />
  ) : (
    <Alert
      message="正在连接本地核心服务"
      description={<Spin size="small" />}
      type="info"
      showIcon
    />
  );
}
