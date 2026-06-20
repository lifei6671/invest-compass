import { randomBytes } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import http from "node:http";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

import { verifyDesktopPackage } from "./verify-desktop-package.mjs";

const tokenHeader = "X-Invest-Compass-Token";
const expectedProtocolVersion = "1";

export async function verifyPackagedSidecarRuntime({
  platform = process.platform,
  packagePath,
  target = "",
  workspacePath = "",
  readyTimeoutMs = 8000,
} = {}) {
  const packageResult = await verifyDesktopPackage({ platform, packagePath, target });
  let temporaryWorkspace = "";
  const resolvedWorkspace =
    workspacePath ||
    (temporaryWorkspace = await mkdtemp(join(tmpdir(), "invest-compass-sidecar-smoke-")));
  try {
    return await verifySidecarRuntime({
      binaryPath: packageResult.sidecarBinary,
      workspacePath: resolvedWorkspace,
      readyTimeoutMs,
    });
  } finally {
    if (temporaryWorkspace) {
      await rm(temporaryWorkspace, { recursive: true, force: true });
    }
  }
}

export async function verifySidecarRuntime({
  binaryPath,
  binaryArgs = [],
  workspacePath,
  readyTimeoutMs = 8000,
  token = randomRuntimeToken(),
  postInternalFn = postInternal,
} = {}) {
  if (!binaryPath || typeof binaryPath !== "string") {
    throw new Error("binaryPath is required");
  }
  if (!workspacePath || typeof workspacePath !== "string") {
    throw new Error("workspacePath is required");
  }
  const child = spawn(binaryPath, [
    ...binaryArgs,
    "--host",
    "127.0.0.1",
    "--port",
    "0",
    "--workspace",
    workspacePath,
  ], {
    stdio: ["pipe", "pipe", "pipe"],
  });
  let childExited = false;
  let stderr = "";
  child.stderr?.setEncoding("utf8");
  child.stderr?.on("data", (chunk) => {
    stderr = `${stderr}${chunk}`.slice(-4096);
  });
  child.once("exit", () => {
    childExited = true;
  });

  try {
    const readyPromise = waitForReadyMessage(child, readyTimeoutMs);
    child.stdin?.write(`${JSON.stringify({ token, protocolVersion: expectedProtocolVersion })}\n`);
    child.stdin?.end();
    const ready = await readyPromise;
    if (ready.protocolVersion !== expectedProtocolVersion) {
      throw new Error(
        `sidecar protocol mismatch: expected ${expectedProtocolVersion}, got ${ready.protocolVersion || "<missing>"}`,
      );
    }
    const health = await postInternalFn(ready.port, "/internal/health", token, readyTimeoutMs);
    const shutdown = await postInternalFn(ready.port, "/internal/shutdown", token, readyTimeoutMs);
    await waitForExit(child, readyTimeoutMs);
    return {
      pid: ready.pid,
      port: ready.port,
      protocolVersion: ready.protocolVersion,
      health,
      shutdown,
    };
  } catch (error) {
    if (stderr.trim()) {
      error.message = `${error.message}; stderr: ${stderr.trim()}`;
    }
    throw error;
  } finally {
    if (!childExited) {
      await terminateChild(child, 2000);
    }
  }
}

function waitForReadyMessage(child, timeoutMs) {
  return new Promise((resolveReady, rejectReady) => {
    let settled = false;
    let stdout = "";
    const timeout = setTimeout(() => {
      rejectOnce(new Error("sidecar ready timeout"));
    }, timeoutMs);

    function cleanup() {
      clearTimeout(timeout);
      child.stdout?.off("data", onData);
      child.off("exit", onExit);
      child.off("error", onError);
    }
    function resolveOnce(value) {
      if (settled) {
        return;
      }
      settled = true;
      cleanup();
      resolveReady(value);
    }
    function rejectOnce(error) {
      if (settled) {
        return;
      }
      settled = true;
      cleanup();
      rejectReady(error);
    }
    function onData(chunk) {
      stdout += chunk.toString("utf8");
      for (;;) {
        const lineBreak = stdout.indexOf("\n");
        if (lineBreak < 0) {
          return;
        }
        const line = stdout.slice(0, lineBreak).trim();
        stdout = stdout.slice(lineBreak + 1);
        if (!line) {
          continue;
        }
        let payload;
        try {
          payload = JSON.parse(line);
        } catch {
          continue;
        }
        if (payload.status === "ready") {
          if (!Number.isInteger(payload.port) || payload.port <= 0) {
            rejectOnce(new Error("sidecar ready message missing port"));
            return;
          }
          resolveOnce(payload);
          return;
        }
      }
    }
    function onExit(code) {
      rejectOnce(new Error(`sidecar exited before ready: ${code ?? "signal"}`));
    }
    function onError(error) {
      rejectOnce(error);
    }
    child.stdout?.on("data", onData);
    child.once("exit", onExit);
    child.once("error", onError);
  });
}

function postInternal(port, path, token, timeoutMs) {
  return new Promise((resolveResponse, rejectResponse) => {
    const request = http.request(
      {
        host: "127.0.0.1",
        port,
        path,
        method: "POST",
        headers: {
          [tokenHeader]: token,
          "X-Request-Id": "sidecar-smoke",
          "X-Trace-Id": "sidecar-smoke",
        },
        timeout: timeoutMs,
      },
      (response) => {
        let body = "";
        response.setEncoding("utf8");
        response.on("data", (chunk) => {
          body += chunk;
        });
        response.on("end", () => {
          let payload;
          try {
            payload = JSON.parse(body);
          } catch {
            rejectResponse(new Error(`invalid JSON response from ${path}`));
            return;
          }
          if (response.statusCode !== 200 || payload.code !== 0) {
            rejectResponse(
              new Error(
                `sidecar ${path} failed: status=${response.statusCode} code=${payload.code} message=${payload.message}`,
              ),
            );
            return;
          }
          resolveResponse(payload);
        });
      },
    );
    request.on("timeout", () => {
      request.destroy(new Error(`sidecar ${path} timeout`));
    });
    request.on("error", rejectResponse);
    request.end();
  });
}

export function waitForExit(child, timeoutMs) {
  if (child.exitCode !== null || child.signalCode !== null) {
    return Promise.resolve();
  }
  return new Promise((resolveExit, rejectExit) => {
    const timeout = setTimeout(() => {
      rejectExit(new Error("sidecar shutdown timeout"));
    }, timeoutMs);
    child.once("exit", () => {
      clearTimeout(timeout);
      resolveExit();
    });
  });
}

async function terminateChild(child, timeoutMs) {
  child.kill("SIGTERM");
  try {
    await waitForExit(child, timeoutMs);
  } catch {
    child.kill("SIGKILL");
  }
}

function randomRuntimeToken() {
  return randomBytes(16).toString("hex");
}

function readCLIOptions(argv) {
  const options = {
    platform: process.platform,
    packagePath: "",
    target: "",
    workspacePath: "",
    readyTimeoutMs: 8000,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--") {
      continue;
    }
    if (arg.startsWith("--platform=")) {
      options.platform = arg.slice("--platform=".length).trim();
      continue;
    }
    if (arg === "--platform") {
      options.platform = readFlagValue(argv, index, "platform");
      index += 1;
      continue;
    }
    if (arg.startsWith("--target=")) {
      options.target = arg.slice("--target=".length).trim();
      continue;
    }
    if (arg === "--target") {
      options.target = readFlagValue(argv, index, "target");
      index += 1;
      continue;
    }
    if (arg.startsWith("--workspace=")) {
      options.workspacePath = resolve(arg.slice("--workspace=".length).trim());
      continue;
    }
    if (arg === "--workspace") {
      options.workspacePath = resolve(readFlagValue(argv, index, "workspace"));
      index += 1;
      continue;
    }
    if (arg.startsWith("--timeout-ms=")) {
      options.readyTimeoutMs = Number.parseInt(arg.slice("--timeout-ms=".length), 10);
      continue;
    }
    if (arg === "--timeout-ms") {
      options.readyTimeoutMs = Number.parseInt(readFlagValue(argv, index, "timeout-ms"), 10);
      index += 1;
      continue;
    }
    if (!options.packagePath) {
      options.packagePath = arg;
      continue;
    }
    throw new Error(`unexpected argument: ${arg}`);
  }
  if (!Number.isInteger(options.readyTimeoutMs) || options.readyTimeoutMs <= 0) {
    throw new Error("timeout-ms must be a positive integer");
  }
  return options;
}

function readFlagValue(argv, index, name) {
  if (!argv[index + 1] || argv[index + 1] === "--") {
    throw new Error(`${name} is required`);
  }
  return argv[index + 1]?.trim() ?? "";
}

async function main() {
  try {
    const result = await verifyPackagedSidecarRuntime(readCLIOptions(process.argv.slice(2)));
    console.log(`sidecar pid: ${result.pid}`);
    console.log(`sidecar port: ${result.port}`);
    console.log(`protocolVersion: ${result.protocolVersion}`);
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

if (resolve(process.argv[1] ?? "") === fileURLToPath(import.meta.url)) {
  await main();
}
