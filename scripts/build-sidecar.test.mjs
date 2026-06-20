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
    "-o",
    "/tmp/invest-compas-core",
    "./cmd/invest-compass-core",
  ]);
});

test("buildGoEnvironment enables CGO for sqlite-backed sidecar targets", () => {
  const [target] = selectBuildTargets(["--target=x86_64-pc-windows-msvc"], "darwin", "arm64");
  const env = buildGoEnvironment(target, { PATH: "/usr/bin", CGO_ENABLED: "0" });

  assert.equal(env.GOOS, "windows");
  assert.equal(env.GOARCH, "amd64");
  assert.equal(env.CGO_ENABLED, "1");
  assert.equal(env.PATH, "/usr/bin");
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
