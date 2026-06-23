import assert from "node:assert/strict";
import { readdir, readFile, stat } from "node:fs/promises";
import { test } from "node:test";

const tauriConfig = JSON.parse(
  await readFile(new URL("../src-tauri/tauri.conf.json", import.meta.url), "utf8"),
);
const packageJson = JSON.parse(await readFile(new URL("../../package.json", import.meta.url), "utf8"));
const frontendPackageJson = JSON.parse(
  await readFile(new URL("../../frontend/package.json", import.meta.url), "utf8"),
);
const cargoToml = await readFile(new URL("../src-tauri/Cargo.toml", import.meta.url), "utf8");
const mainCapability = JSON.parse(
  await readFile(
    new URL("../src-tauri/capabilities/default.json", import.meta.url),
    "utf8",
  ),
);
const libSource = await readFile(new URL("../src-tauri/src/lib.rs", import.meta.url), "utf8");
const desktopRuntimeSource = await readFile(
  new URL("../src-tauri/src/desktop_runtime.rs", import.meta.url),
  "utf8",
);
const commandSources = await readCommandSources(
  new URL("../src-tauri/src/commands/", import.meta.url),
);
const rendererBoundarySources = await readSourceFiles([
  new URL("../../frontend/src/", import.meta.url),
  new URL("../../packages/shared/src/", import.meta.url),
], (fileName) =>
  /\.(ts|tsx)$/.test(fileName) &&
  !fileName.includes(".test.") &&
  !fileName.includes(".spec."),
);
const goCoreBoundarySources = await readSourceFiles(
  [new URL("../../sidecar-core/", import.meta.url)],
  (fileName, fileUrl) =>
    fileName.endsWith(".go") &&
    !fileName.endsWith("_test.go") &&
    !fileUrl.pathname.includes("/internal/dao/"),
);
const goServerSources = await readSourceFiles(
  [new URL("../../sidecar-core/internal/server/", import.meta.url)],
  (fileName) => fileName.endsWith(".go") && !fileName.endsWith("_test.go"),
);
const goDaoSources = await readSourceFiles(
  [new URL("../../sidecar-core/internal/dao/", import.meta.url)],
  (fileName) => fileName.endsWith(".go") && !fileName.endsWith("_test.go"),
);
const goServiceSourceEntries = await readSourceFileEntries(
  [new URL("../../sidecar-core/internal/service/", import.meta.url)],
  (fileName) => fileName.endsWith(".go") && !fileName.endsWith("_test.go"),
);
const goServiceSources = goServiceSourceEntries.map((entry) => entry.source);

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

test("main capability 只包含首版需要的 core、日志目录选择和任务通知权限", () => {
  assert.deepEqual(mainCapability.windows, ["main"]);
  assert.deepEqual(mainCapability.permissions, [
    "core:default",
    "core:window:default",
    "core:event:default",
    "dialog:allow-open",
    "notification:allow-is-permission-granted",
    "notification:allow-request-permission",
    "notification:allow-notify",
  ]);

  const dialogPermissions = mainCapability.permissions.filter((permission) =>
    permission.startsWith("dialog:"),
  );
  assert.deepEqual(
    dialogPermissions,
    ["dialog:allow-open"],
    "设置页只需要原生目录选择，不应开放保存或消息弹窗权限",
  );

  const notificationPermissions = mainCapability.permissions.filter((permission) =>
    permission.startsWith("notification:"),
  );
  assert.deepEqual(
    notificationPermissions,
    [
      "notification:allow-is-permission-granted",
      "notification:allow-request-permission",
      "notification:allow-notify",
    ],
    "任务通知只需要检查权限、请求权限和发送通知，不能开放批量或默认全量通知权限",
  );

  const forbiddenPluginPrefixes = [
    "clipboard-manager:",
    "fs:",
    "global-shortcut:",
    "http:",
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

  assert.equal(
    mainCapability.permissions.includes("notification:default"),
    false,
    "默认 capability 不能使用 notification:default，避免扩大桌面通知能力面",
  );
});

test("Tauri 通知插件必须完整接入并保持最小权限", () => {
  assert.match(
    frontendPackageJson.dependencies?.["@tauri-apps/plugin-notification"],
    /^\^2\./,
    "前端必须通过官方 notification 插件发送任务终态通知",
  );
  assert.match(
    cargoToml,
    /tauri-plugin-notification\s*=\s*"2"/,
    "Rust 侧必须初始化官方 notification 插件，不能只在前端调用不存在的能力",
  );
  assert.match(
    libSource,
    /\.plugin\(tauri_plugin_notification::init\(\)\)/,
    "Tauri Builder 必须注册 notification 插件",
  );
});

test("开机自启只能通过 Rust 白名单命令调用官方插件", () => {
  assert.match(
    cargoToml,
    /tauri-plugin-autostart\s*=\s*"2\.\d+\.\d+"/,
    "Rust 侧必须接入官方 autostart 插件，不能自研平台启动项写入逻辑",
  );
  assert.match(
    libSource,
    /tauri_plugin_autostart::init\(\s*tauri_plugin_autostart::MacosLauncher::LaunchAgent,\s*None,\s*\)/s,
    "Tauri Builder 必须初始化 autostart 插件并使用 macOS LaunchAgent",
  );

  const commands = parseTauriCommandNames(commandSources);
  assert.equal(commands.includes("autostart_get"), true, "必须提供读取开机自启状态的白名单命令");
  assert.equal(commands.includes("autostart_set"), true, "必须提供设置开机自启状态的白名单命令");
  assert.equal(
    mainCapability.permissions.some((permission) => permission.startsWith("autostart:")),
    false,
    "前端不直接调用 autostart guest API，因此默认 capability 不能开放 autostart 插件权限",
  );
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

test("桌面运行期必须提供真实托盘恢复入口", () => {
  assert.match(
    cargoToml,
    /tauri\s*=\s*\{[^\n]*features\s*=\s*\[[^\]]*"tray-icon"/,
    "关闭到托盘依赖 Tauri tray-icon feature，不能只在代码里假定托盘可用",
  );
  assert.match(
    desktopRuntimeSource,
    /TrayIconBuilder::new\(\)/,
    "desktop_runtime 必须创建真实托盘图标，避免关闭后无恢复入口",
  );
  assert.match(desktopRuntimeSource, /"open-workbench"/, "托盘菜单必须包含打开工作台入口");
  assert.match(desktopRuntimeSource, /"quit-app"/, "托盘菜单必须包含退出应用入口");
});

test("Tauri externalBin 必须随包声明 Go sidecar", () => {
  assert.deepEqual(
    tauriConfig.bundle?.externalBin,
    ["binaries/invest-compass-core"],
    "externalBin 必须使用 Tauri sidecar 基名，实际文件由构建脚本补 target triple 后缀",
  );
  assert.match(
    packageJson.scripts?.["sidecar:build"] ?? "",
    /scripts\/build-sidecar\.mjs/,
    "sidecar:build 必须通过专用脚本产出 target triple sidecar 文件",
  );
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

test("settings 和 cache Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    [
      "cache_clean",
      "cache_stats",
      "settings_get",
      "settings_set",
      "workspace_get",
      "workspace_set",
    ].filter((command) => !commands.includes(command)),
    [],
    "settings/cache/workspace command 必须显式声明，不能用通用代理代替",
  );

  assert.deepEqual(findMissingFixedCommandPaths(commandSources), []);
});

test("股票搜索 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.equal(commands.includes("stock_search"), true, "stock_search command 必须显式声明");

  const combinedSource = commandSources.join("\n");
  assert.equal(
    combinedSource.includes('"/api/stocks/search"'),
    true,
    "stock_search 必须固定映射到 /api/stocks/search",
  );
});

test("行情 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    ["market_quote", "market_kline", "market_indicators"].filter(
      (command) => !commands.includes(command),
    ),
    [],
    "market quote/kline/indicators command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of ["/api/market/quote", "/api/market/kline", "/api/market/indicators"]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `行情 command 必须固定映射到 ${path}`,
    );
  }
});

test("自选股 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    [
      "watchlist_create",
      "watchlist_delete",
      "watchlist_list",
      "watchlist_update",
    ].filter((command) => !commands.includes(command)),
    [],
    "watchlist command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of [
    "/api/watchlist/list",
    "/api/watchlist/create",
    "/api/watchlist/update",
    "/api/watchlist/delete",
  ]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `watchlist command 必须固定映射到 ${path}`,
    );
  }
});

test("新闻 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    ["news_list", "news_market"].filter((command) => !commands.includes(command)),
    [],
    "news command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of ["/api/news/list", "/api/news/market"]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `news command 必须固定映射到 ${path}`,
    );
  }
});

test("Dashboard 和 Provider 状态 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    ["dashboard_summary", "providers_status"].filter((command) => !commands.includes(command)),
    [],
    "dashboard/provider command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of ["/api/dashboard/summary", "/api/providers/status"]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `dashboard/provider command 必须固定映射到 ${path}`,
    );
  }
});

test("Prompt 模板 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    [
      "prompt_templates_create",
      "prompt_templates_delete",
      "prompt_templates_get",
      "prompt_templates_list",
      "prompt_templates_update",
    ].filter((command) => !commands.includes(command)),
    [],
    "prompt template command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of [
    "/api/prompt-templates/list",
    "/api/prompt-templates/get",
    "/api/prompt-templates/create",
    "/api/prompt-templates/update",
    "/api/prompt-templates/delete",
  ]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `prompt template command 必须固定映射到 ${path}`,
    );
  }
});

test("AI 配置 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    ["ai_config_delete", "ai_config_list", "ai_config_save", "ai_config_test"].filter(
      (command) => !commands.includes(command),
    ),
    [],
    "AI config command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of [
    "/api/ai/configs/list",
    "/api/ai/configs/save",
    "/api/ai/configs/delete",
    "/api/ai/configs/test",
  ]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `AI config command 必须固定映射到 ${path}`,
    );
  }
});

test("AI 配置连通性测试不能阻塞 Tauri 同步 command", () => {
  const aiConfigSource = commandSources.find((source) =>
    source.includes("pub async fn ai_config_test"),
  );
  assert.notEqual(
    aiConfigSource,
    undefined,
    "ai_config_test 必须是 async command，避免外部模型网络测试阻塞桌面 UI",
  );
  assert.match(
    aiConfigSource,
    /tauri::async_runtime::spawn_blocking/,
    "ai_config_test 中的阻塞式 Go core 请求必须放到 blocking 线程执行",
  );
});

test("任务历史 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    [
      "analysis_task_cancel",
      "analysis_task_create",
      "analysis_task_subscribe",
      "task_events",
      "task_get",
      "task_list",
    ].filter((command) => !commands.includes(command)),
    [],
    "task command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of [
    "/api/analysis/tasks",
    "/api/tasks/cancel",
    "/api/tasks/events/stream",
    "/api/tasks/list",
    "/api/tasks/get",
    "/api/tasks/events",
  ]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `task history command 必须固定映射到 ${path}`,
    );
  }
});

test("报告历史 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    ["report_delete", "report_get", "report_list"].filter((command) => !commands.includes(command)),
    [],
    "report history command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of ["/api/reports/list", "/api/reports/get", "/api/reports/delete"]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `report history command 必须固定映射到 ${path}`,
    );
  }
});

test("菜单范围搜索 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    [
      "search_news",
      "search_rebuild",
      "search_reports",
      "search_status",
      "search_watchlist_notes",
    ].filter((command) => !commands.includes(command)),
    [],
    "menu-scoped search command 必须显式声明，不能用通用代理或全局搜索代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of [
    "/api/search/reports",
    "/api/search/news",
    "/api/search/watchlist-notes",
    "/api/search/status",
    "/api/search/rebuild",
  ]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `menu-scoped search command 必须固定映射到 ${path}`,
    );
  }
  assert.equal(combinedSource.includes("search_global"), false, "首版不允许 search_global command");
  assert.equal(combinedSource.includes("/api/search/global"), false, "首版不允许全局搜索 API");
  assert.equal(
    /\bsearch_request\b|\bsearch_global_request\b/i.test(combinedSource),
    false,
    "搜索 command 不能退化为通用 search_request 代理",
  );
});

test("检查更新 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.equal(commands.includes("check_update"), true, "check_update command 必须显式声明");

  const combinedSource = commandSources.join("\n");
  assert.equal(
    combinedSource.includes('"/api/update/check"'),
    true,
    "check_update 必须固定映射到 /api/update/check",
  );
});

test("日志导出 Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.equal(commands.includes("export_logs"), true, "export_logs command 必须显式声明");

  const combinedSource = commandSources.join("\n");
  assert.equal(
    combinedSource.includes('"/api/logs/export"'),
    true,
    "export_logs 必须固定映射到 /api/logs/export",
  );
});

test("Scheduler Rust command 必须固定映射到 Go API", () => {
  const commands = parseTauriCommandNames(commandSources);
  assert.deepEqual(
    [
      "scheduler_job_types",
      "scheduler_jobs_backfill",
      "scheduler_jobs_delete",
      "scheduler_jobs_get",
      "scheduler_jobs_list",
      "scheduler_jobs_run_now",
      "scheduler_jobs_save",
      "scheduler_jobs_set_enabled",
      "scheduler_refresh_symbol",
      "scheduler_runs_get",
      "scheduler_runs_list",
      "scheduler_runs_trigger",
      "scheduler_status",
    ].filter((command) => !commands.includes(command)),
    [],
    "scheduler command 必须显式声明，不能用通用代理代替",
  );

  const combinedSource = commandSources.join("\n");
  for (const path of [
    "/api/scheduler/job-types",
    "/api/scheduler/jobs/backfill",
    "/api/scheduler/jobs/get",
    "/api/scheduler/jobs/list",
    "/api/scheduler/jobs/run-now",
    "/api/scheduler/jobs/save",
    "/api/scheduler/jobs/set-enabled",
    "/api/scheduler/jobs/delete",
    "/api/scheduler/refresh-symbol",
    "/api/scheduler/runs/get",
    "/api/scheduler/runs/list",
    "/api/scheduler/runs/trigger",
    "/api/scheduler/status",
  ]) {
    assert.equal(
      combinedSource.includes(`"${path}"`),
      true,
      `scheduler command 必须固定映射到 ${path}`,
    );
  }
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

test("前端生产源码禁止首版未闭环入口文案", () => {
  assert.deepEqual(findRendererNonMVPEntryUsage(rendererBoundarySources), []);
});

test("前端和共享契约禁止绕过 typed invoke service 直接取数", () => {
  assert.deepEqual(findRendererDataSourceBypass(rendererBoundarySources), []);
});

test("Go core 非 dao 层禁止手写 SQL 或直接使用 database/sql", () => {
  assert.deepEqual(findGoDaoBoundaryLeaks(goCoreBoundarySources), []);
});

test("Go core actions 路由注册层必须使用 Gin 框架", async () => {
  const routerSource = await readFile(
    new URL("../../sidecar-core/internal/actions/router.go", import.meta.url),
    "utf8",
  );

  assert.equal(
    routerSource.includes("\"github.com/gin-gonic/gin\""),
    true,
    "internal/actions/router.go 必须使用 github.com/gin-gonic/gin 承载本地 HTTP API 路由注册",
  );
});

test("Go HTTP server 路由注册和业务 handler 必须拆到 actions", async () => {
  const sidecarRoot = new URL("../../sidecar-core/", import.meta.url);
  const routerUrl = new URL("internal/actions/router.go", sidecarRoot);

  assert.equal(await pathExists(routerUrl), true, "internal/actions/router.go 必须集中注册 HTTP 路由");
  assert.deepEqual(findGoServerHTTPBoundaryLeaks(goServerSources), []);

  const actionRouterSource = await readFile(routerUrl, "utf8");
  assert.equal(
    actionRouterSource.includes("\"github.com/gin-gonic/gin\""),
    true,
    "actions/router.go 必须作为唯一 Gin 路由注册入口",
  );

  const actionSubpackageSources = await readSourceFiles(
    [new URL("../../sidecar-core/internal/actions/", import.meta.url)],
    (fileName, fileUrl) =>
      fileName.endsWith(".go") &&
      !fileName.endsWith("_test.go") &&
      !fileUrl.pathname.endsWith("/internal/actions/router.go"),
  );
  assert.deepEqual(findGoActionSubpackageRouterLeaks(actionSubpackageSources), []);
});

test("Go dao 层必须使用 GORM 组件", () => {
  assert.equal(
    goDaoSources.some((source) => source.includes("\"gorm.io/gorm\"")),
    true,
    "internal/dao 必须使用 gorm.io/gorm 作为数据库访问入口",
  );
});

test("Go core 必须保留 service dao model pkg/constant 和 pkg/xerr 分层目录", async () => {
  const sidecarRoot = new URL("../../sidecar-core/", import.meta.url);
  const requiredLayerDocs = [
    "internal/service/doc.go",
    "internal/dao/doc.go",
    "internal/model/doc.go",
    "pkg/constant/doc.go",
    "pkg/xerr/doc.go",
  ];

  for (const layerDoc of requiredLayerDocs) {
    const source = await readFile(new URL(layerDoc, sidecarRoot), "utf8");
    assert.match(source, /^\/\/ Package /, `${layerDoc} 必须提供包职责说明`);
  }
});

test("Go core 业务模块必须迁移到 service 子包", async () => {
  const sidecarRoot = new URL("../../sidecar-core/", import.meta.url);
  const servicePackages = [
    "analysis",
    "ai",
    "dashboard",
    "indicator",
    "logexport",
    "market",
    "news",
    "prompt",
    "report",
    "settings",
    "sidecar",
    "stock",
    "task",
    "updatecheck",
    "watchlist",
  ];

  for (const packageName of servicePackages) {
    assert.equal(
      await pathExists(new URL(`internal/service/${packageName}/doc.go`, sidecarRoot)),
      true,
      `业务包必须放在 internal/service/${packageName}`,
    );
    assert.equal(
      await pathExists(new URL(`internal/${packageName}/doc.go`, sidecarRoot)),
      false,
      `业务包不能继续留在 internal/${packageName}`,
    );
  }

  assert.equal(await pathExists(new URL("pkg/logger/doc.go", sidecarRoot)), true);
  assert.equal(await pathExists(new URL("internal/logger/doc.go", sidecarRoot)), false);
});

test("Go service 层禁止定义通用错误码和错误结构", async () => {
  const sidecarRoot = new URL("../../sidecar-core/", import.meta.url);

  assert.equal(await pathExists(new URL("pkg/xerr/doc.go", sidecarRoot)), true);
  assert.deepEqual(findGoServiceErrorDefinitionLeaks(goServiceSources), []);
});

test("Go gocron 调度库只能由 scheduler service 使用", () => {
  const leaks = goServiceSourceEntries
    .filter((entry) => entry.source.includes("github.com/go-co-op/gocron/v2"))
    .filter((entry) => !entry.path.includes("/internal/service/scheduler/"))
    .map((entry) => entry.path);

  assert.deepEqual(leaks, []);
});

test("Go Provider service 不能直接写调度表或调度模型", () => {
  const leaks = goServiceSourceEntries
    .filter((entry) => !entry.path.includes("/internal/service/scheduler/"))
    .filter((entry) => /\bScheduler(Job|Run)\b|\bIngestionWatermark\b|scheduler_jobs|scheduler_runs|ingestion_watermarks/.test(entry.source))
    .map((entry) => entry.path);

  assert.deepEqual(leaks, []);
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

test("Go dao 边界扫描能识别 handler 中的 SQL 和 database/sql", () => {
  const unsafeSource = `
    package server

    import "database/sql"

    func listStocks(db *sql.DB) {
      db.Query("SELECT * FROM stocks WHERE deleted_at IS NULL")
    }
  `;

  assert.deepEqual(findGoDaoBoundaryLeaks([unsafeSource]), [
    "go source 在非 dao 层直接使用 database/sql",
    "go source 在非 dao 层手写 SQL 语句",
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

test("前端未闭环入口扫描能识别非首版入口", () => {
  const unsafeSource = `
    <nav>
      <a>策略观察</a>
      <button>授权激活</button>
      <button>资金流</button>
    </nav>
  `;

  assert.deepEqual(findRendererNonMVPEntryUsage([unsafeSource]), [
    "renderer source 包含首版未闭环入口：策略观察",
    "renderer source 包含首版未闭环入口：授权激活",
    "renderer source 包含首版未闭环入口：资金流",
  ]);
});

async function readCommandSources(directoryUrl) {
  return readSourceFiles([directoryUrl], (fileName) => fileName.endsWith(".rs"));
}

async function readSourceFiles(directoryUrls, includeFile = () => true) {
  const entries = await readSourceFileEntries(directoryUrls, includeFile);
  return entries.map((entry) => entry.source);
}

async function readSourceFileEntries(directoryUrls, includeFile = () => true) {
  const entries = [];

  for (const directoryUrl of directoryUrls) {
    entries.push(...(await readSourceFileEntriesFromDirectory(directoryUrl, includeFile)));
  }

  return entries;
}

async function readSourceFileEntriesFromDirectory(directoryUrl, includeFile) {
  const entries = await readdir(directoryUrl, { withFileTypes: true });
  const sources = [];

  for (const entry of entries) {
    const entryUrl = new URL(entry.name, directoryUrl);
    if (entry.isDirectory()) {
      sources.push(
        ...(await readSourceFileEntriesFromDirectory(new URL(`${entry.name}/`, directoryUrl), includeFile)),
      );
    } else if (includeFile(entry.name, entryUrl)) {
      sources.push({ path: entryUrl.pathname, source: await readFile(entryUrl, "utf8") });
    }
  }

  return sources;
}

async function pathExists(fileUrl) {
  try {
    await stat(fileUrl);
    return true;
  } catch (error) {
    if (error?.code === "ENOENT") {
      return false;
    }
    throw error;
  }
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

function findMissingFixedCommandPaths(sources) {
  const combinedSource = sources.join("\n");
  const requiredPaths = [
    "/api/settings/get",
    "/api/settings/set",
    "/api/workspace/get",
    "/api/workspace/set",
    "/api/cache/stats",
    "/api/cache/clean",
  ];

  return requiredPaths
    .filter((path) => !combinedSource.includes(`"${path}"`))
    .map((path) => `缺少固定 Go API path ${path}`);
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

function findGoDaoBoundaryLeaks(sources) {
  const checks = [
    {
      pattern: /"database\/sql"|\bdatabase\/sql\b/,
      message: "go source 在非 dao 层直接使用 database/sql",
    },
    {
      pattern: /\b(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP)\b[\s\S]*\b(FROM|INTO|TABLE|SET|VALUES)\b/i,
      message: "go source 在非 dao 层手写 SQL 语句",
    },
  ];

  return sources.flatMap((source) =>
    checks
      .filter((check) => check.pattern.test(source))
      .map((check) => check.message),
  );
}

function findGoServerHTTPBoundaryLeaks(sources) {
  const checks = [
    {
      pattern: /"github\.com\/gin-gonic\/gin"/,
      message: "server package 不能直接依赖 Gin",
    },
    {
      pattern: /\.(POST|GET|PUT|PATCH|DELETE|Handle)\s*\(/,
      message: "server package 不能注册业务路由",
    },
    {
      pattern: /"\/(?:api|internal)\//,
      message: "server package 不能硬编码业务 API path",
    },
    {
      pattern: /\bhandle[A-Z][A-Za-z0-9_]*\s*\(/,
      message: "server package 不能承载业务 handler",
    },
  ];

  return sources.flatMap((source) =>
    checks
      .filter((check) => check.pattern.test(source))
      .map((check) => check.message),
  );
}

function findGoActionSubpackageRouterLeaks(sources) {
  const checks = [
    {
      pattern: /"github\.com\/gin-gonic\/gin"/,
      message: "actions 子包不能直接依赖 Gin",
    },
    {
      pattern: /\.(POST|GET|PUT|PATCH|DELETE|Handle)\s*\(/,
      message: "actions 子包不能直接注册 Gin 路由",
    },
  ];

  return sources.flatMap((source) =>
    checks
      .filter((check) => check.pattern.test(source))
      .map((check) => check.message),
  );
}

function findGoServiceErrorDefinitionLeaks(sources) {
  const checks = [
    {
      pattern: /\btype\s+(ErrorCode|Code)\s+string\b/,
      message: "service source 定义了错误码类型，应迁移到 pkg/xerr",
    },
    {
      pattern: /\btype\s+Error\s+struct\s*\{[\s\S]*?\bCode\b/,
      message: "service source 定义了通用 Error 结构，应迁移到 pkg/xerr",
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

function findRendererNonMVPEntryUsage(sources) {
  const forbiddenLabels = [
    "策略观察",
    "授权激活",
    "公告",
    "研报",
    "资金流",
    "券商账户",
    "自动下单",
    "云同步",
    "移动端",
  ];

  return sources.flatMap((source) =>
    forbiddenLabels
      .filter((label) => source.includes(label))
      .map((label) => `renderer source 包含首版未闭环入口：${label}`),
  );
}
