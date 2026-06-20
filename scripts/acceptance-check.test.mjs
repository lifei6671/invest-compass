import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const packageJson = JSON.parse(
  await readFile(new URL("../apps/package.json", import.meta.url), "utf8"),
);

test("apps workspace exposes one local release acceptance check command", () => {
  const script = packageJson.scripts?.["acceptance:check"] ?? "";

  assert.match(script, /\bpnpm\s+test\b/, "acceptance:check 必须复跑测试基线");
  assert.match(script, /\bpnpm\s+check\b/, "acceptance:check 必须复跑类型和编译检查");
  assert.match(script, /\bpnpm\s+desktop:test\b/, "acceptance:check 必须复跑 Rust 桌面单元测试");
  assert.match(script, /\bpnpm\s+format:check\b/, "acceptance:check 必须复跑格式检查");
  assert.doesNotMatch(script, /\b(provider:smoke|tauri\s+build|package:verify)\b/, "acceptance:check 不能隐式执行联网或真实打包验收");
});

test("apps workspace exposes one local release candidate check command", () => {
  const script = packageJson.scripts?.["release:check:local"] ?? "";

  assert.match(script, /\bpnpm\s+acceptance:check\b/, "release:check:local 必须先复跑本地验收基线");
  assert.match(script, /\bpnpm\s+build\b/, "release:check:local 必须重新生成前端和 sidecar 构建产物");
  assert.match(script, /tauri\s+build\s+--bundles\s+app/, "release:check:local 必须重新生成 macOS .app 包");
  assert.match(script, /\bpnpm\s+package:verify\b/, "release:check:local 必须复核生成包结构");
  assert.match(script, /\bpnpm\s+sidecar:smoke\b/, "release:check:local 必须验证包内 Go sidecar 可握手启动和关闭");
  assert.match(script, /\bpnpm\s+sqlite:upgrade-rehearsal\b/, "release:check:local 必须执行 SQLite 发布升级演练");
  assert.match(script, /\bpnpm\s+sidecar:check-targets\b/, "release:check:local 必须执行三类 sidecar target 产物校验");
  assert.match(script, /--platform=darwin/, "release:check:local 当前只覆盖本机 macOS 包复核");
  assert.match(script, /--target=aarch64-apple-darwin/, "release:check:local 必须复核 Apple Silicon 目标架构");
  assert.match(script, /git\s+-C\s+\.\.\s+diff\s+--check/, "release:check:local 必须在仓库根目录执行空白差异检查");
  assert.doesNotMatch(script, /\bprovider:smoke\b/, "release:check:local 不能隐式触发联网 Provider smoke");
});

test("apps workspace exposes desktop Rust unit test command", () => {
  const script = packageJson.scripts?.["desktop:test"] ?? "";

  assert.equal(
    script,
    "cargo test --manifest-path desktop/src-tauri/Cargo.toml",
    "desktop:test 必须运行 Tauri Rust crate 的单元测试",
  );
});

test("apps workspace sidecar tests do not modify Go module files", () => {
  const script = packageJson.scripts?.["sidecar:test"] ?? "";

  assert.match(script, /\bgo\s+test\s+-mod=readonly\s+\.\//, "sidecar:test 必须禁止 go test 自动改写 go.mod/go.sum");
});

test("apps workspace exposes packaged sidecar runtime smoke command", () => {
  const script = packageJson.scripts?.["sidecar:smoke"] ?? "";

  assert.match(script, /verify-sidecar-runtime\.mjs/, "sidecar:smoke 必须使用专用 sidecar runtime smoke 脚本");
  assert.match(script, /--platform=darwin/, "sidecar:smoke 当前只覆盖本机 macOS 包内 sidecar");
  assert.match(script, /--target=aarch64-apple-darwin/, "sidecar:smoke 必须复核 Apple Silicon 包内 sidecar");
  assert.match(script, /投研罗盘\.app/, "sidecar:smoke 必须指向生成后的首版 macOS app bundle");
});

test("apps workspace exposes all sidecar target artifact check command", () => {
  const verifyScript = packageJson.scripts?.["sidecar:verify-targets"] ?? "";
  const checkScript = packageJson.scripts?.["sidecar:check-targets"] ?? "";

  assert.match(
    verifyScript,
    /verify-sidecar-targets\.mjs/,
    "sidecar:verify-targets 必须复核三类首版 sidecar target 产物",
  );
  assert.match(
    checkScript,
    /\bpnpm\s+sidecar:build\s+--\s+--all-targets\b/,
    "sidecar:check-targets 必须重新构建全部首版 sidecar target",
  );
  assert.match(
    checkScript,
    /\bpnpm\s+sidecar:verify-targets\b/,
    "sidecar:check-targets 必须在构建后复核 target 架构和 Windows 子系统",
  );
});

test("apps workspace exposes SQLite upgrade rehearsal command", () => {
  const script = packageJson.scripts?.["sqlite:upgrade-rehearsal"] ?? "";

  assert.match(
    script,
    /cd\s+sidecar-core\s+&&\s+go\s+test\s+-mod=readonly\s+\.\//,
    "sqlite:upgrade-rehearsal 必须在 Go module 根目录运行且禁止改写 go.mod/go.sum",
  );
  assert.match(
    script,
    /TestBackupBeforeMigrationBackupCanBeRestoredAndMigrated/,
    "sqlite:upgrade-rehearsal 必须覆盖迁移前备份可恢复并重新迁移",
  );
  assert.match(script, /-count=1\b/, "sqlite:upgrade-rehearsal 必须禁用 Go test 缓存");
});
