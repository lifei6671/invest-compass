import { copyFileSync, mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

export const supportedTargets = new Map([
  [
    "aarch64-apple-darwin",
    {
      goos: "darwin",
      goarch: "arm64",
      fileName: "invest-compas-core-aarch64-apple-darwin",
      compatName: "invest-compas-core",
    },
  ],
  [
    "x86_64-apple-darwin",
    {
      goos: "darwin",
      goarch: "amd64",
      fileName: "invest-compas-core-x86_64-apple-darwin",
      compatName: "invest-compas-core",
    },
  ],
  [
    "x86_64-pc-windows-msvc",
    {
      goos: "windows",
      goarch: "amd64",
      fileName: "invest-compas-core-x86_64-pc-windows-msvc.exe",
      compatName: "invest-compas-core.exe",
    },
  ],
]);

const platformTargetTriples = new Map([
  ["darwin:arm64", "aarch64-apple-darwin"],
  ["darwin:x64", "x86_64-apple-darwin"],
  ["win32:x64", "x86_64-pc-windows-msvc"],
]);

export function selectBuildTargets(argv, platform = process.platform, arch = process.arch) {
  const targetArg = readTargetArg(argv);
  if (argv.includes("--all-targets")) {
    return Array.from(supportedTargets, ([triple, target]) => ({ triple, ...target }));
  }
  if (targetArg) {
    const target = supportedTargets.get(targetArg);
    if (!target) {
      throw new Error(`unsupported sidecar target: ${targetArg}`);
    }
    return [{ triple: targetArg, ...target }];
  }

  const currentTriple = platformTargetTriples.get(`${platform}:${arch}`);
  if (!currentTriple) {
    throw new Error(`unsupported sidecar build platform: ${platform}:${arch}`);
  }
  const target = supportedTargets.get(currentTriple);
  return [{ triple: currentTriple, ...target }];
}

export function buildGoCommand(target, outputPath) {
  const args = ["build", "-mod=readonly"];
  if (target.goos === "windows") {
    args.push("-ldflags", "-H windowsgui");
  }
  args.push("-o", outputPath, "./cmd/invest-compass-core");
  return { command: "go", args };
}

export function buildGoEnvironment(target, baseEnv = process.env) {
  return {
    ...baseEnv,
    GOOS: target.goos,
    GOARCH: target.goarch,
    CGO_ENABLED: "1",
  };
}

export function assertSidecarBundleNames(externalBin) {
  if (!Array.isArray(externalBin) || externalBin.length !== 1) {
    throw new Error("tauri externalBin must declare exactly one sidecar binary base");
  }
  const bundleBaseName = externalBin[0]?.split("/").pop();
  if (!bundleBaseName) {
    throw new Error("tauri externalBin sidecar base name is empty");
  }
  for (const [triple, target] of supportedTargets) {
    const expectedFileName = `${bundleBaseName}-${triple}${target.goos === "windows" ? ".exe" : ""}`;
    if (target.fileName !== expectedFileName) {
      throw new Error(
        `sidecar target ${triple} file name ${target.fileName} must match ${expectedFileName}`,
      );
    }
  }
}

function readTargetArg(argv) {
  const inline = argv.find((arg) => arg.startsWith("--target="));
  if (inline) {
    return inline.slice("--target=".length).trim();
  }
  const targetFlagIndex = argv.indexOf("--target");
  if (targetFlagIndex >= 0) {
    return argv[targetFlagIndex + 1]?.trim() ?? "";
  }
  return "";
}

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..");
const sidecarDir = join(repoRoot, "apps", "sidecar-core");
const binariesDir = join(repoRoot, "apps", "desktop", "src-tauri", "binaries");
const currentPlatformTriple = platformTargetTriples.get(`${process.platform}:${process.arch}`);

function buildSidecar(target) {
  mkdirSync(binariesDir, { recursive: true });
  const outputPath = join(binariesDir, target.fileName);
  const command = buildGoCommand(target, outputPath);

  const result = spawnSync(command.command, command.args, {
    cwd: sidecarDir,
    env: buildGoEnvironment(target),
    stdio: "inherit",
  });

  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }

  if (target.triple === currentPlatformTriple) {
    copyFileSync(outputPath, join(binariesDir, target.compatName));
  }
  console.log(`built sidecar: ${outputPath}`);
}

function main() {
  try {
    for (const target of selectBuildTargets(process.argv.slice(2))) {
      buildSidecar(target);
    }
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

if (resolve(process.argv[1] ?? "") === fileURLToPath(import.meta.url)) {
  main();
}
