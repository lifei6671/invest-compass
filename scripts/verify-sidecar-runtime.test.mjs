import test from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { chmod, mkdtemp, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";

import { verifySidecarRuntime, waitForExit } from "./verify-sidecar-runtime.mjs";

async function writeFakeSidecar(root, { protocolVersion = "1" } = {}) {
  const scriptPath = join(root, "fake-sidecar.mjs");
  await writeFile(
    scriptPath,
    `let buffer = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => {
  buffer += chunk;
  if (!buffer.includes("\\n")) {
    return;
  }
  JSON.parse(buffer.trim());
  console.log(JSON.stringify({
    status: "ready",
    port: 58123,
    pid: process.pid,
    protocolVersion: "${protocolVersion}"
  }));
  setTimeout(() => process.exit(0), 120);
});
`,
  );
  await chmod(scriptPath, 0o700);
  return scriptPath;
}

test("verifySidecarRuntime completes handshake health and shutdown", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-sidecar-runtime-"));
  try {
    const fakeSidecar = await writeFakeSidecar(root);

    const result = await verifySidecarRuntime({
      binaryPath: process.execPath,
      binaryArgs: [fakeSidecar],
      workspacePath: root,
      readyTimeoutMs: 3000,
      token: "runtime-test-token",
      postInternalFn: async (port, path, token) => {
        assert.equal(port, 58123);
        assert.equal(token, "runtime-test-token");
        if (path === "/internal/health") {
          return { code: 0, message: "ok", data: { version: "0.1.0", dbStatus: "ok" } };
        }
        if (path === "/internal/shutdown") {
          return { code: 0, message: "ok", data: { status: "shutting_down" } };
        }
        throw new Error(`unexpected path ${path}`);
      },
    });

    assert.equal(result.protocolVersion, "1");
    assert.equal(result.health.data.version, "0.1.0");
    assert.equal(result.shutdown.data.status, "shutting_down");
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifySidecarRuntime rejects protocol mismatches", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-sidecar-protocol-"));
  try {
    const fakeSidecar = await writeFakeSidecar(root, { protocolVersion: "2" });

    await assert.rejects(
      () =>
        verifySidecarRuntime({
          binaryPath: process.execPath,
          binaryArgs: [fakeSidecar],
          workspacePath: root,
          readyTimeoutMs: 3000,
          token: "runtime-test-token",
          postInternalFn: async () => {
            throw new Error("protocol mismatch should fail before HTTP checks");
          },
        }),
      /sidecar protocol mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("waitForExit resolves when the sidecar already exited before listener registration", async () => {
  const child = new EventEmitter();
  child.exitCode = 0;
  child.signalCode = null;

  await waitForExit(child, 10);
});
