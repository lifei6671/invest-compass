import { lstat, readFile, stat } from "node:fs/promises";
import { basename, extname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const projectRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));

const platformLayouts = {
  darwin: {
    desktopBinary: ["Contents", "MacOS", "invest-compass-desktop"],
    sidecarBinary: ["Contents", "MacOS", "invest-compass-core"],
  },
  win32: {
    desktopBinary: ["invest-compass-desktop.exe"],
    sidecarBinary: ["invest-compass-core.exe"],
  },
};

const targetArchitectures = {
  "aarch64-apple-darwin": {
    platform: "darwin",
    format: "macho",
    cpuType: 0x0100000c,
    label: "Mach-O arm64",
  },
  "x86_64-apple-darwin": {
    platform: "darwin",
    format: "macho",
    cpuType: 0x01000007,
    label: "Mach-O x86_64",
  },
  "x86_64-pc-windows-msvc": {
    platform: "win32",
    format: "pe",
    machine: 0x8664,
    subsystem: 2,
    label: "PE x86_64",
    subsystemLabel: "Windows GUI subsystem",
  },
};

export async function verifyDesktopPackage({
  platform = process.platform,
  packagePath,
  target = "",
} = {}) {
  if (!packagePath || typeof packagePath !== "string") {
    throw new Error("packagePath is required");
  }
  const layout = platformLayouts[platform];
  if (!layout) {
    throw new Error(`unsupported desktop package platform: ${platform}`);
  }
  const targetArchitecture = resolveTargetArchitecture({ platform, target });
  const expectedMacOSMetadata = platform === "darwin" ? await readExpectedMacOSMetadata() : null;

  const resolvedPackagePath = resolve(packagePath);
  await assertPackageRoot(resolvedPackagePath);
  if (platform === "darwin" && extname(resolvedPackagePath) !== ".app") {
    throw new Error(`macOS app bundle path must end with .app: ${resolvedPackagePath}`);
  }
  if (platform === "darwin") {
    assertMacOSAppBundleName(resolvedPackagePath, expectedMacOSMetadata);
  }
  const desktopBinary = join(resolvedPackagePath, ...layout.desktopBinary);
  const sidecarBinary = join(resolvedPackagePath, ...layout.sidecarBinary);
  if (platform === "darwin") {
    await assertDirectory(join(resolvedPackagePath, "Contents"), "macOS Contents directory");
    await assertDirectory(join(resolvedPackagePath, "Contents", "MacOS"), "macOS executable directory");
    await assertMacOSInfoPlist(resolvedPackagePath, basename(desktopBinary), expectedMacOSMetadata);
  }

  await assertFile(desktopBinary, {
    missingMessage: "desktop binary is missing",
    emptyMessage: "desktop binary is empty",
    executableMessage: platform === "darwin" ? "desktop binary is not executable" : "",
  });
  await assertFile(sidecarBinary, {
    missingMessage: "packaged sidecar binary is missing",
    emptyMessage: "packaged sidecar binary is empty",
    executableMessage: platform === "darwin" ? "packaged sidecar binary is not executable" : "",
  });
  if (targetArchitecture) {
    await assertArchitecture(desktopBinary, targetArchitecture, "desktop binary");
    await assertArchitecture(sidecarBinary, targetArchitecture, "packaged sidecar binary");
  }

  return {
    platform,
    desktopBinary,
    sidecarBinary,
  };
}

export async function verifyBinaryArchitecture({
  path,
  target,
  platform = "",
  binaryLabel = "binary",
} = {}) {
  if (!path || typeof path !== "string") {
    throw new Error("path is required");
  }
  const targetArchitecture = resolveTargetArchitecture({ platform, target });
  if (!targetArchitecture) {
    throw new Error("target is required");
  }
  await assertArchitecture(path, targetArchitecture, binaryLabel);
  return {
    path,
    target,
    label: targetArchitecture.label,
  };
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

async function assertPackageRoot(path) {
  let metadata;
  try {
    metadata = await lstat(path);
  } catch {
    throw new Error(`desktop package root is missing: ${path}`);
  }
  if (metadata.isSymbolicLink()) {
    throw new Error(`desktop package root must not be a symlink: ${path}`);
  }
  if (!metadata.isDirectory()) {
    throw new Error(`desktop package root must be a directory: ${path}`);
  }
}

async function assertFile(path, { missingMessage, emptyMessage, executableMessage = "" }) {
  let linkMetadata;
  try {
    linkMetadata = await lstat(path);
  } catch {
    throw new Error(`${missingMessage}: ${path}`);
  }
  if (linkMetadata.isSymbolicLink()) {
    throw new Error(
      `${missingMessage.replace("is missing", "must be packaged as a regular file")}: ${path}`,
    );
  }
  let metadata;
  try {
    metadata = await stat(path);
  } catch {
    throw new Error(`${missingMessage}: ${path}`);
  }
  if (!metadata.isFile()) {
    throw new Error(`${missingMessage}: ${path}`);
  }
  if (metadata.size === 0) {
    throw new Error(`${emptyMessage}: ${path}`);
  }
  if (executableMessage && (metadata.mode & 0o111) === 0) {
    throw new Error(`${executableMessage}: ${path}`);
  }
}

async function readExpectedMacOSMetadata() {
  const configPath = join(projectRoot, "apps", "desktop", "src-tauri", "tauri.conf.json");
  const config = JSON.parse(await readFile(configPath, "utf8"));
  if (!config.identifier || typeof config.identifier !== "string") {
    throw new Error(`Tauri config missing identifier: ${configPath}`);
  }
  if (!config.version || typeof config.version !== "string") {
    throw new Error(`Tauri config missing version: ${configPath}`);
  }
  if (!config.productName || typeof config.productName !== "string") {
    throw new Error(`Tauri config missing productName: ${configPath}`);
  }
  return {
    identifier: config.identifier.trim(),
    productName: config.productName.trim(),
    version: config.version.trim(),
  };
}

function assertMacOSAppBundleName(appBundlePath, expectedMetadata) {
  const expectedName = `${expectedMetadata.productName}.app`;
  const actualName = basename(appBundlePath);
  if (actualName !== expectedName) {
    throw new Error(
      `macOS app bundle name mismatch: expected ${expectedName}, got ${actualName}: ${appBundlePath}`,
    );
  }
}

async function assertMacOSInfoPlist(appBundlePath, expectedExecutable, expectedMetadata) {
  const infoPlistPath = join(appBundlePath, "Contents", "Info.plist");
  await assertFile(infoPlistPath, {
    missingMessage: "macOS Info.plist is missing",
    emptyMessage: "macOS Info.plist is empty",
  });
  let infoPlist;
  try {
    infoPlist = await readFile(infoPlistPath, "utf8");
  } catch {
    throw new Error(`macOS Info.plist is missing: ${infoPlistPath}`);
  }
  const executable = readPlistString(infoPlist, "CFBundleExecutable");
  if (!executable) {
    throw new Error(`macOS Info.plist missing CFBundleExecutable: ${infoPlistPath}`);
  }
  if (executable !== expectedExecutable) {
    throw new Error(
      `macOS Info.plist CFBundleExecutable mismatch: expected ${expectedExecutable}, got ${executable}: ${infoPlistPath}`,
    );
  }
  const packageType = readPlistString(infoPlist, "CFBundlePackageType");
  if (packageType !== "APPL") {
    throw new Error(
      `macOS Info.plist CFBundlePackageType mismatch: expected APPL, got ${packageType || "<missing>"}: ${infoPlistPath}`,
    );
  }
  const displayName = readPlistString(infoPlist, "CFBundleDisplayName");
  if (displayName !== expectedMetadata.productName) {
    throw new Error(
      `macOS Info.plist CFBundleDisplayName mismatch: expected ${expectedMetadata.productName}, got ${displayName || "<missing>"}: ${infoPlistPath}`,
    );
  }
  const bundleName = readPlistString(infoPlist, "CFBundleName");
  if (bundleName !== expectedMetadata.productName) {
    throw new Error(
      `macOS Info.plist CFBundleName mismatch: expected ${expectedMetadata.productName}, got ${bundleName || "<missing>"}: ${infoPlistPath}`,
    );
  }
  const bundleIdentifier = readPlistString(infoPlist, "CFBundleIdentifier");
  if (!bundleIdentifier) {
    throw new Error(`macOS Info.plist missing CFBundleIdentifier: ${infoPlistPath}`);
  }
  if (bundleIdentifier !== expectedMetadata.identifier) {
    throw new Error(
      `macOS Info.plist CFBundleIdentifier mismatch: expected ${expectedMetadata.identifier}, got ${bundleIdentifier}: ${infoPlistPath}`,
    );
  }
  const shortVersion = readPlistString(infoPlist, "CFBundleShortVersionString");
  if (shortVersion !== expectedMetadata.version) {
    throw new Error(
      `macOS Info.plist CFBundleShortVersionString mismatch: expected ${expectedMetadata.version}, got ${shortVersion || "<missing>"}: ${infoPlistPath}`,
    );
  }
  const bundleVersion = readPlistString(infoPlist, "CFBundleVersion");
  if (bundleVersion !== expectedMetadata.version) {
    throw new Error(
      `macOS Info.plist CFBundleVersion mismatch: expected ${expectedMetadata.version}, got ${bundleVersion || "<missing>"}: ${infoPlistPath}`,
    );
  }
  const iconFile = readPlistString(infoPlist, "CFBundleIconFile");
  if (iconFile) {
    const resourcesDir = join(appBundlePath, "Contents", "Resources");
    assertMacOSIconFileName(iconFile);
    await assertMacOSResourcesDirectory(resourcesDir, iconFile);
    await assertFile(join(resourcesDir, iconFile), {
      missingMessage: "macOS icon resource is missing",
      emptyMessage: "macOS icon resource is empty",
    });
  }
}

function assertMacOSIconFileName(iconFile) {
  if (iconFile === "." || iconFile === ".." || /[\\/]/.test(iconFile)) {
    throw new Error(
      `macOS icon resource path must stay inside Contents/Resources: ${iconFile}`,
    );
  }
}

async function assertMacOSResourcesDirectory(resourcesDir, iconFile) {
  let metadata;
  try {
    metadata = await lstat(resourcesDir);
  } catch {
    throw new Error(`macOS icon resource is missing: ${join(resourcesDir, iconFile)}`);
  }
  if (metadata.isSymbolicLink()) {
    throw new Error(`macOS Resources directory must not be a symlink: ${resourcesDir}`);
  }
  if (!metadata.isDirectory()) {
    throw new Error(`macOS Resources directory must be a directory: ${resourcesDir}`);
  }
}

function readPlistString(plist, key) {
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = plist.match(new RegExp(`<key>\\s*${escapedKey}\\s*</key>\\s*<string>([^<]+)</string>`));
  return match?.[1]?.trim() ?? "";
}

function resolveTargetArchitecture({ platform, target }) {
  if (!target) {
    return null;
  }
  const architecture = targetArchitectures[target];
  if (!architecture) {
    throw new Error(`unsupported desktop package target: ${target}`);
  }
  if (platform && architecture.platform !== platform) {
    throw new Error(`target ${target} does not match platform ${platform}`);
  }
  return architecture;
}

async function assertArchitecture(path, targetArchitecture, binaryLabel) {
  const buffer = await readFile(path);
  if (targetArchitecture.format === "macho") {
    if (!hasMachOCpuType(buffer, targetArchitecture.cpuType)) {
      throw new Error(
        `${binaryLabel} architecture mismatch: expected ${targetArchitecture.label}: ${path}`,
      );
    }
    return;
  }

  const peHeader = readPEHeader(buffer);
  if (!peHeader || peHeader.machine !== targetArchitecture.machine) {
    throw new Error(
      `${binaryLabel} architecture mismatch: expected ${targetArchitecture.label}: ${path}`,
    );
  }
  if (targetArchitecture.subsystem && peHeader.subsystem !== targetArchitecture.subsystem) {
    throw new Error(
      `${binaryLabel} subsystem mismatch: expected ${targetArchitecture.subsystemLabel}: ${path}`,
    );
  }
}

function hasMachOCpuType(buffer, expectedCpuType) {
  if (buffer.length < 8) {
    return false;
  }
  const littleEndianMagic = buffer.readUInt32LE(0);
  if (littleEndianMagic === 0xfeedfacf || littleEndianMagic === 0xfeedface) {
    return buffer.readInt32LE(4) === expectedCpuType;
  }
  const bigEndianMagic = buffer.readUInt32BE(0);
  if (bigEndianMagic === 0xfeedfacf || bigEndianMagic === 0xfeedface) {
    return buffer.readInt32BE(4) === expectedCpuType;
  }
  if (bigEndianMagic === 0xcafebabe || bigEndianMagic === 0xcafebabf) {
    return hasFatMachOCpuType(buffer, expectedCpuType, bigEndianMagic === 0xcafebabf);
  }
  return false;
}

function hasFatMachOCpuType(buffer, expectedCpuType, uses64BitArchHeader) {
  if (buffer.length < 8) {
    return false;
  }
  const archCount = buffer.readUInt32BE(4);
  const archSize = uses64BitArchHeader ? 32 : 20;
  for (let index = 0; index < archCount; index += 1) {
    const archOffset = 8 + index * archSize;
    if (buffer.length < archOffset + 4) {
      return false;
    }
    if (buffer.readInt32BE(archOffset) === expectedCpuType) {
      return true;
    }
  }
  return false;
}

function readPEHeader(buffer) {
  if (buffer.length < 0x40 || buffer.readUInt16LE(0) !== 0x5a4d) {
    return null;
  }
  const peOffset = buffer.readUInt32LE(0x3c);
  if (buffer.length < peOffset + 6) {
    return null;
  }
  if (buffer.readUInt32LE(peOffset) !== 0x00004550) {
    return null;
  }
  const optionalHeaderOffset = peOffset + 24;
  if (buffer.length < optionalHeaderOffset + 70) {
    return null;
  }
  return {
    machine: buffer.readUInt16LE(peOffset + 4),
    subsystem: buffer.readUInt16LE(optionalHeaderOffset + 68),
  };
}

function readCLIOptions(argv) {
  const options = {
    platform: process.platform,
    packagePath: "",
    target: "",
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
      if (!argv[index + 1] || argv[index + 1] === "--") {
        throw new Error("platform is required");
      }
      options.platform = argv[index + 1]?.trim() ?? "";
      index += 1;
      continue;
    }
    if (arg.startsWith("--target=")) {
      options.target = arg.slice("--target=".length).trim();
      continue;
    }
    if (arg === "--target") {
      if (!argv[index + 1] || argv[index + 1] === "--") {
        throw new Error("target is required");
      }
      options.target = argv[index + 1]?.trim() ?? "";
      index += 1;
      continue;
    }
    if (!options.packagePath) {
      options.packagePath = arg;
      continue;
    }
    throw new Error(`unexpected argument: ${arg}`);
  }
  return options;
}

async function main() {
  try {
    const result = await verifyDesktopPackage(readCLIOptions(process.argv.slice(2)));
    console.log(`desktop binary: ${result.desktopBinary}`);
    console.log(`sidecar binary: ${result.sidecarBinary}`);
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

if (resolve(process.argv[1] ?? "") === fileURLToPath(import.meta.url)) {
  await main();
}
