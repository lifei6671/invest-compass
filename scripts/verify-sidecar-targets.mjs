import { lstat, readFile, stat } from "node:fs/promises";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { supportedTargets } from "./build-sidecar.mjs";
import { verifyBinaryArchitecture } from "./verify-desktop-package.mjs";

const projectRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const defaultBinariesDir = join(projectRoot, "apps", "desktop", "src-tauri", "binaries");

export async function verifySidecarTargets({ binariesDir = defaultBinariesDir } = {}) {
  const resolvedBinariesDir = resolve(binariesDir);
  await assertDirectory(resolvedBinariesDir, "sidecar binaries directory");

  const results = [];
  for (const [target, metadata] of supportedTargets) {
    const binaryPath = join(resolvedBinariesDir, metadata.fileName);
    await assertSidecarBinary(binaryPath, {
      target,
      requiresExecutablePermission: metadata.goos !== "windows",
    });
    const architecture = await verifyBinaryArchitecture({
      path: binaryPath,
      target,
      binaryLabel: `sidecar target ${target}`,
    });
    await assertUsableSQLiteBuild(binaryPath, { target });
    results.push({
      target,
      path: binaryPath,
      label: architecture.label,
    });
  }
  return results;
}

async function assertUsableSQLiteBuild(path, { target }) {
  const content = await readFile(path);
  if (content.includes(Buffer.from("CGO_ENABLED=0", "utf8"))) {
    throw new Error(`sidecar target ${target} was built with CGO_ENABLED=0 and cannot use go-sqlite3`);
  }
}

async function assertDirectory(path, label) {
  let metadata;
  try {
    metadata = await lstat(path);
  } catch {
    throw new Error(`${label} is missing: ${path}`);
  }
  if (metadata.isSymbolicLink()) {
    throw new Error(`${label} must not be a symlink: ${path}`);
  }
  if (!metadata.isDirectory()) {
    throw new Error(`${label} must be a directory: ${path}`);
  }
}

async function assertSidecarBinary(path, { target, requiresExecutablePermission }) {
  let linkMetadata;
  try {
    linkMetadata = await lstat(path);
  } catch {
    throw new Error(`sidecar target binary is missing: ${target}: ${path}`);
  }
  if (linkMetadata.isSymbolicLink()) {
    throw new Error(`sidecar target binary must be a regular file: ${target}: ${path}`);
  }

  const metadata = await stat(path);
  if (!metadata.isFile()) {
    throw new Error(`sidecar target binary is missing: ${target}: ${path}`);
  }
  if (metadata.size === 0) {
    throw new Error(`sidecar target binary is empty: ${target}: ${path}`);
  }
  if (requiresExecutablePermission && (metadata.mode & 0o111) === 0) {
    throw new Error(`sidecar target binary is not executable: ${target}: ${path}`);
  }
}

function readCLIOptions(argv) {
  const options = {
    binariesDir: defaultBinariesDir,
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--") {
      continue;
    }
    if (arg.startsWith("--binaries-dir=")) {
      options.binariesDir = arg.slice("--binaries-dir=".length).trim();
      continue;
    }
    if (arg === "--binaries-dir") {
      if (!argv[index + 1] || argv[index + 1] === "--") {
        throw new Error("binaries-dir is required");
      }
      options.binariesDir = argv[index + 1]?.trim() ?? "";
      index += 1;
      continue;
    }
    throw new Error(`unexpected argument: ${arg}`);
  }
  return options;
}

async function main() {
  try {
    const results = await verifySidecarTargets(readCLIOptions(process.argv.slice(2)));
    for (const result of results) {
      console.log(`sidecar target ${result.target}: ${result.path} (${result.label})`);
    }
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

if (resolve(process.argv[1] ?? "") === fileURLToPath(import.meta.url)) {
  await main();
}
