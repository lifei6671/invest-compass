import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

import { buildGoCommand, selectBuildTargets } from "./build-sidecar.mjs";

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
  const command = buildGoCommand(target, "/tmp/invest-compass-core.exe");

  assert.deepEqual(command.args, [
    "build",
    "-ldflags",
    "-H windowsgui",
    "-o",
    "/tmp/invest-compass-core.exe",
    "./cmd/invest-compass-core",
  ]);
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
    /go build -o \.\.\/desktop\/src-tauri\/binaries\/invest-compass-core /,
    "go-build must not emit only the legacy non-target sidecar filename",
  );
});
