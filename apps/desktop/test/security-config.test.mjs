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

async function readCommandSources(directoryUrl) {
  const entries = await readdir(directoryUrl, { withFileTypes: true });
  const sources = [];

  for (const entry of entries) {
    const entryUrl = new URL(entry.name, directoryUrl);
    if (entry.isDirectory()) {
      sources.push(...(await readCommandSources(new URL(`${entry.name}/`, directoryUrl))));
    } else if (entry.name.endsWith(".rs")) {
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
