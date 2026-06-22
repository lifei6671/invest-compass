import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

import { assertSidecarBundleNames, buildGoCommand, buildGoEnvironment, selectBuildTargets } from "./build-sidecar.mjs";

test("selectBuildTargets defaults to current platform target", () => {
  const targets = selectBuildTargets([], "darwin", "arm64");

  assert.deepEqual(
    targets.map((target) => target.triple),
    ["aarch64-apple-darwin"],
  );
});

test("selectBuildTargets supports explicit target triple", () => {
  const targets = selectBuildTargets(["--target=x86_64-pc-windows-msvc"], "darwin", "arm64");

  assert.deepEqual(
    targets.map((target) => target.triple),
    ["x86_64-pc-windows-msvc"],
  );
});

test("selectBuildTargets supports all first-version desktop targets", () => {
  const targets = selectBuildTargets(["--all-targets"], "darwin", "arm64");

  assert.deepEqual(
    targets.map((target) => target.triple),
    ["aarch64-apple-darwin", "x86_64-apple-darwin", "x86_64-pc-windows-msvc"],
  );
});

test("buildGoCommand uses GUI subsystem for Windows sidecar", () => {
  const [target] = selectBuildTargets(["--target=x86_64-pc-windows-msvc"], "darwin", "arm64");
  const command = buildGoCommand(target, "/tmp/invest-compas-core.exe");

  assert.deepEqual(command.args, [
    "build",
    "-mod=readonly",
    "-tags",
    "sqlite_fts5",
    "-ldflags",
    "-H windowsgui",
    "-o",
    "/tmp/invest-compas-core.exe",
    "./cmd/invest-compass-core",
  ]);
});

test("buildGoCommand does not modify Go module files", () => {
  const [target] = selectBuildTargets(["--target=aarch64-apple-darwin"], "darwin", "arm64");
  const command = buildGoCommand(target, "/tmp/invest-compas-core");

  assert.deepEqual(command.args, [
    "build",
    "-mod=readonly",
    "-tags",
    "sqlite_fts5",
    "-o",
    "/tmp/invest-compas-core",
    "./cmd/invest-compass-core",
  ]);
});

test("buildGoCommand enables SQLite FTS5 for every sidecar target", () => {
  const targets = selectBuildTargets(["--all-targets"], "darwin", "arm64");

  for (const target of targets) {
    const command = buildGoCommand(target, `/tmp/${target.fileName}`);

    assert.deepEqual(
      command.args.slice(0, 4),
      ["build", "-mod=readonly", "-tags", "sqlite_fts5"],
      `${target.triple} sidecar build must enable SQLite FTS5`,
    );
  }
});

test("buildGoEnvironment enables CGO for sqlite-backed sidecar targets", () => {
  const [target] = selectBuildTargets(["--target=x86_64-pc-windows-msvc"], "darwin", "arm64");
  const env = buildGoEnvironment(target, { PATH: "/usr/bin", CGO_ENABLED: "0" });

  assert.equal(env.GOOS, "windows");
  assert.equal(env.GOARCH, "amd64");
  assert.equal(env.CGO_ENABLED, "1");
  assert.equal(env.CC, "x86_64-w64-mingw32-gcc");
  assert.equal(env.PATH, "/usr/bin");
});

test("buildGoEnvironment preserves explicit Windows CGO compiler override", () => {
  const [target] = selectBuildTargets(["--target=x86_64-pc-windows-msvc"], "darwin", "arm64");
  const env = buildGoEnvironment(target, { PATH: "/usr/bin", CC: "custom-windows-gcc" });

  assert.equal(env.CC, "custom-windows-gcc");
});

test("buildGoEnvironment configures macOS CGO arch flags for cross-arch sidecar targets", () => {
  const [target] = selectBuildTargets(["--target=x86_64-apple-darwin"], "darwin", "arm64");
  const env = buildGoEnvironment(
    target,
    { PATH: "/usr/bin", CGO_CFLAGS: "-O2", CGO_LDFLAGS: "-s" },
    "darwin",
    "arm64",
  );

  assert.equal(env.CC, "clang");
  assert.match(env.CGO_CFLAGS, /-arch x86_64\b/);
  assert.match(env.CGO_CFLAGS, /-O2\b/);
  assert.match(env.CGO_LDFLAGS, /-arch x86_64\b/);
  assert.match(env.CGO_LDFLAGS, /-s\b/);
});

test("Makefile delegates sidecar build to target-aware script", async () => {
  const makefile = await readFile(new URL("../Makefile", import.meta.url), "utf8");

  assert.match(
    makefile,
    /go-build:\n\tpnpm --dir \$\(APPS_DIR\) sidecar:build/,
    "go-build must use scripts/build-sidecar.mjs through pnpm sidecar:build",
  );
  assert.doesNotMatch(
    makefile,
    /go build -o \.\.\/desktop\/src-tauri\/binaries\/invest-compas-core /,
    "go-build must not emit only the legacy non-target sidecar filename",
  );
});

test("sidecar target 文件名必须匹配 Tauri externalBin 基名", async () => {
  const tauriConfig = JSON.parse(
    await readFile(new URL("../apps/desktop/src-tauri/tauri.conf.json", import.meta.url), "utf8"),
  );

  assert.doesNotThrow(() => assertSidecarBundleNames(tauriConfig.bundle?.externalBin));
});
