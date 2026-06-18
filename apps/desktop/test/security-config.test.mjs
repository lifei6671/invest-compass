import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { test } from "node:test";

const tauriConfig = JSON.parse(
  await readFile(new URL("../src-tauri/tauri.conf.json", import.meta.url), "utf8"),
);
const mainCapability = JSON.parse(
  await readFile(
    new URL("../src-tauri/capabilities/default.json", import.meta.url),
    "utf8",
  ),
);
const libSource = await readFile(new URL("../src-tauri/src/lib.rs", import.meta.url), "utf8");
const commandSources = await readCommandSources(
  new URL("../src-tauri/src/commands/", import.meta.url),
);
const rendererBoundarySources = await readSourceFiles([
  new URL("../../frontend/src/", import.meta.url),
  new URL("../../packages/shared/src/", import.meta.url),
]);
const goCoreBoundarySources = await readSourceFiles(
  [new URL("../../sidecar-core/", import.meta.url)],
  (fileName, fileUrl) =>
    fileName.endsWith(".go") &&
    !fileName.endsWith("_test.go") &&
    !fileUrl.pathname.includes("/internal/storage/"),
);

test("Tauri 主窗口显式绑定 main capability", () => {
  assert.deepEqual(
    tauriConfig.app?.security?.capabilities,
    ["main"],
    "tauri.conf.json 必须显式只启用 main capability",
  );
  assert.equal(
    tauriConfig.app?.windows?.[0]?.label,
    "main",
    "主窗口 label 必须与 capability windows 声明一致",
  );
});

test("main capability 只包含首版需要的 core 权限", () => {
  assert.deepEqual(mainCapability.windows, ["main"]);
  assert.deepEqual(mainCapability.permissions, [
    "core:default",
    "core:window:default",
    "core:event:default",
  ]);

  const forbiddenPluginPrefixes = [
    "clipboard-manager:",
    "dialog:",
    "fs:",
    "global-shortcut:",
    "http:",
    "notification:",
    "shell:",
    "store:",
    "updater:",
  ];

  for (const permission of mainCapability.permissions) {
    assert.equal(
      forbiddenPluginPrefixes.some((prefix) => permission.startsWith(prefix)),
      false,
      `未启用的插件权限不能进入默认 capability: ${permission}`,
    );
  }
});

test("CSP 不允许任意远程脚本或弱化脚本执行策略", () => {
  const csp = tauriConfig.app?.security?.csp ?? "";

  assert.match(csp, /default-src 'self'/);
  assert.doesNotMatch(csp, /script-src[^;]*\*/);
  assert.doesNotMatch(csp, /script-src[^;]*https?:/);
  assert.doesNotMatch(csp, /'unsafe-eval'/);
  assert.doesNotMatch(csp, /default-src[^;]*\*/);
  assert.doesNotMatch(csp, /default-src[^;]*https?:/);
});

test("Rust command 必须显式注册到 invoke_handler 白名单", () => {
  assert.deepEqual(
    parseGenerateHandlerCommands(libSource),
    parseTauriCommandNames(commandSources),
  );
});

test("Rust command 禁止实现任意路径 core_request 代理", () => {
  assert.deepEqual(findArbitraryProxyCommands(commandSources), []);
});

test("Rust command 安全扫描能识别任意 method/path/body 代理", () => {
  const unsafeSource = `
    #[tauri::command]
    pub async fn core_request(method: String, path: String, body: serde_json::Value) {}
  `;

  assert.deepEqual(findArbitraryProxyCommands([unsafeSource]), [
    "core_request 暴露 method/path/body 任意代理参数",
  ]);
});

test("前端和共享契约禁止暴露 Go core、runtime token 或内部密钥字段", () => {
  assert.deepEqual(findRendererBoundaryLeaks(rendererBoundarySources), []);
});

test("前端生产源码禁止 mock 数据伪装真实能力", () => {
  assert.deepEqual(findRendererMockDataUsage(rendererBoundarySources), []);
});

test("前端和共享契约禁止绕过 typed invoke service 直接取数", () => {
  assert.deepEqual(findRendererDataSourceBypass(rendererBoundarySources), []);
});

test("Go core 非 storage 层禁止手写 SQL 或直接使用 database/sql", () => {
  assert.deepEqual(findGoStorageBoundaryLeaks(goCoreBoundarySources), []);
});

test("前端安全扫描能识别直连 Go core 和内部密钥字段", () => {
  const unsafeSource = `
    const resolved_api_key = "secret";
    const stream = new EventSource("http://127.0.0.1:58123/api/tasks/events/stream");
    localStorage.setItem("token", "secret");
    fetch("http://localhost:58123/internal/health", {
      headers: { "X-Invest-Compass-Token": "secret" },
    });
  `;

  assert.deepEqual(findRendererBoundaryLeaks([unsafeSource]), [
    "renderer source 暴露内部密钥字段 resolved_api_key",
    "renderer source 直接使用 EventSource 连接 Go core",
    "renderer source 使用浏览器持久化存储保存敏感状态",
    "renderer source 直连本地 Go core 地址",
    "renderer source 处理 runtime token header",
  ]);
});

test("前端数据源扫描能识别直接 HTTP 和浏览器网络调用", () => {
  const unsafeSource = `
    const client = axios.create({ baseURL: "https://example.com" });
    const response = await fetch("/api/stocks/search");
    const xhr = new XMLHttpRequest();
  `;

  assert.deepEqual(findRendererDataSourceBypass([unsafeSource]), [
    "renderer source 直接使用 fetch 取数",
    "renderer source 直接使用 XMLHttpRequest 取数",
    "renderer source 直接使用 axios 取数",
    "renderer source 硬编码 HTTP 数据源地址",
  ]);
});

test("Go storage 边界扫描能识别 handler 中的 SQL 和 database/sql", () => {
  const unsafeSource = `
    package server

    import "database/sql"

    func listStocks(db *sql.DB) {
      db.Query("SELECT * FROM stocks WHERE deleted_at IS NULL")
    }
  `;

  assert.deepEqual(findGoStorageBoundaryLeaks([unsafeSource]), [
    "go source 在非 storage 层直接使用 database/sql",
    "go source 在非 storage 层手写 SQL 语句",
  ]);
});

test("前端 mock 数据扫描能识别伪造投研数据入口", () => {
  const unsafeSource = `
    const mockStocks = [{ symbol: "CN:SH:600519", price: 123.45 }];
    const fakeReport = { title: "AI report", content: "demo" };
    const demoTasks = [];
  `;

  assert.deepEqual(findRendererMockDataUsage([unsafeSource]), [
    "renderer source 包含 mock/fake/dummy/fixture 数据入口",
    "renderer source 包含 demo/sample 业务数据入口",
    "renderer source 包含硬编码投研业务数据",
  ]);
});

async function readCommandSources(directoryUrl) {
  return readSourceFiles([directoryUrl], (fileName) => fileName.endsWith(".rs"));
}

async function readSourceFiles(directoryUrls, includeFile = () => true) {
  const sources = [];

  for (const directoryUrl of directoryUrls) {
    sources.push(...(await readSourceFilesFromDirectory(directoryUrl, includeFile)));
  }

  return sources;
}

async function readSourceFilesFromDirectory(directoryUrl, includeFile) {
  const entries = await readdir(directoryUrl, { withFileTypes: true });
  const sources = [];

  for (const entry of entries) {
    const entryUrl = new URL(entry.name, directoryUrl);
    if (entry.isDirectory()) {
      sources.push(
        ...(await readSourceFilesFromDirectory(new URL(`${entry.name}/`, directoryUrl), includeFile)),
      );
    } else if (includeFile(entry.name, entryUrl)) {
      sources.push(await readFile(entryUrl, "utf8"));
    }
  }

  return sources;
}

function parseGenerateHandlerCommands(source) {
  const handlerBlock = source.match(/tauri::generate_handler!\s*\[([\s\S]*?)\]/)?.[1] ?? "";

  return handlerBlock
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .map((item) => item.split("::").at(-1))
    .sort();
}

function parseTauriCommandNames(sources) {
  return sources
    .flatMap((source) => [...source.matchAll(/#\[tauri::command\]\s*(?:\/\/.*\n|\s)*pub\s+(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)/g)])
    .map((match) => match[1])
    .sort();
}

function findArbitraryProxyCommands(sources) {
  const failures = [];
  const commandPattern =
    /#\[tauri::command\]\s*(?:\/\/.*\n|\s)*pub\s+(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([\s\S]*?)\)\s*(?:->|\{)/g;

  for (const source of sources) {
    for (const match of source.matchAll(commandPattern)) {
      const [, commandName, params] = match;
      if (commandName === "core_request" || hasArbitraryProxyParams(params)) {
        failures.push(`${commandName} 暴露 method/path/body 任意代理参数`);
      }
    }
  }

  return failures;
}

function hasArbitraryProxyParams(params) {
  return /\bmethod\s*:/.test(params) && /\bpath\s*:/.test(params) && /\bbody\s*:/.test(params);
}

function findRendererBoundaryLeaks(sources) {
  const checks = [
    {
      pattern: /\bresolved_api_key\b|\braw_api_key\b/,
      message: "renderer source 暴露内部密钥字段 resolved_api_key",
    },
    {
      pattern: /\bnew\s+EventSource\b|\bEventSource\s*\(/,
      message: "renderer source 直接使用 EventSource 连接 Go core",
    },
    {
      pattern: /\blocalStorage\b|\bsessionStorage\b|\bindexedDB\b/,
      message: "renderer source 使用浏览器持久化存储保存敏感状态",
    },
    {
      pattern: /http:\/\/127\.0\.0\.1:\d+|http:\/\/localhost:\d+/,
      message: "renderer source 直连本地 Go core 地址",
    },
    {
      pattern: /\bX-Invest-Compass-Token\b/,
      message: "renderer source 处理 runtime token header",
    },
  ];

  return sources.flatMap((source) =>
    checks
      .filter((check) => check.pattern.test(source))
      .map((check) => check.message),
  );
}

function findRendererDataSourceBypass(sources) {
  const checks = [
    {
      pattern: /\bfetch\s*\(/,
      message: "renderer source 直接使用 fetch 取数",
    },
    {
      pattern: /\bnew\s+XMLHttpRequest\b|\bXMLHttpRequest\s*\(/,
      message: "renderer source 直接使用 XMLHttpRequest 取数",
    },
    {
      pattern: /\baxios\b/,
      message: "renderer source 直接使用 axios 取数",
    },
    {
      pattern: /https?:\/\//,
      message: "renderer source 硬编码 HTTP 数据源地址",
    },
  ];

  return sources.flatMap((source) =>
    checks
      .filter((check) => check.pattern.test(source))
      .map((check) => check.message),
  );
}

function findGoStorageBoundaryLeaks(sources) {
  const checks = [
    {
      pattern: /"database\/sql"|\bdatabase\/sql\b/,
      message: "go source 在非 storage 层直接使用 database/sql",
    },
    {
      pattern: /\b(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP)\b[\s\S]*\b(FROM|INTO|TABLE|SET|VALUES)\b/i,
      message: "go source 在非 storage 层手写 SQL 语句",
    },
  ];

  return sources.flatMap((source) =>
    checks
      .filter((check) => check.pattern.test(source))
      .map((check) => check.message),
  );
}

function findRendererMockDataUsage(sources) {
  const checks = [
    {
      pattern: /\b(mock|fake|dummy|fixture)[A-Za-z0-9_]*\b/i,
      message: "renderer source 包含 mock/fake/dummy/fixture 数据入口",
    },
    {
      pattern: /\b(demo|sample)(Stocks?|Quotes?|Klines?|News|Reports?|Tasks?|Watchlists?|Data)\b/i,
      message: "renderer source 包含 demo/sample 业务数据入口",
    },
    {
      pattern: /\b(CN:(SH|SZ):\d{6}|HK:\d{5}|US:[A-Z]{1,5})\b[\s\S]*\b(price|quote|kline|report|task|news|watchlist|symbol)\b/i,
      message: "renderer source 包含硬编码投研业务数据",
    },
  ];

  return sources.flatMap((source) => {
    const messages = new Set();
    for (const check of checks) {
      if (check.pattern.test(source)) {
        messages.add(check.message);
      }
    }
    return [...messages];
  });
}
