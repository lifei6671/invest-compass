import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { chmod, mkdtemp, mkdir, rm, symlink, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { promisify } from "node:util";

import { verifyDesktopPackage } from "./verify-desktop-package.mjs";

const execFileAsync = promisify(execFile);

function machO64CpuType(cpuType) {
  const buffer = Buffer.alloc(8);
  buffer.writeUInt32LE(0xfeedfacf, 0);
  buffer.writeInt32LE(cpuType, 4);
  return buffer;
}

function infoPlistExecutable(executable) {
  return infoPlistExecutableWithPackageType(executable, "APPL");
}

function infoPlistExecutableWithPackageType(executable, packageType) {
  return infoPlistExecutableWithMetadata({
    executable,
    packageType,
    displayName: "投研罗盘",
    identifier: "com.lifei6671.investcompass",
    name: "投研罗盘",
    shortVersion: "0.1.0",
    bundleVersion: "0.1.0",
  });
}

function infoPlistExecutableWithMetadata({
  executable,
  packageType,
  displayName,
  identifier,
  name,
  shortVersion,
  bundleVersion,
}) {
  return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>${executable}</string>
  <key>CFBundlePackageType</key>
  <string>${packageType}</string>
  ${displayName ? `<key>CFBundleDisplayName</key>\n  <string>${displayName}</string>` : ""}
  ${identifier ? `<key>CFBundleIdentifier</key>\n  <string>${identifier}</string>` : ""}
  ${name ? `<key>CFBundleName</key>\n  <string>${name}</string>` : ""}
  ${shortVersion ? `<key>CFBundleShortVersionString</key>\n  <string>${shortVersion}</string>` : ""}
  ${bundleVersion ? `<key>CFBundleVersion</key>\n  <string>${bundleVersion}</string>` : ""}
</dict>
</plist>
`;
}

function infoPlistExecutableWithIcon(executable, iconFile) {
  return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>${executable}</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleDisplayName</key>
  <string>投研罗盘</string>
  <key>CFBundleIdentifier</key>
  <string>com.lifei6671.investcompass</string>
  <key>CFBundleName</key>
  <string>投研罗盘</string>
  <key>CFBundleShortVersionString</key>
  <string>0.1.0</string>
  <key>CFBundleVersion</key>
  <string>0.1.0</string>
  <key>CFBundleIconFile</key>
  <string>${iconFile}</string>
</dict>
</plist>
`;
}

function peMachine(machine, subsystem = 2) {
  const peOffset = 0x80;
  const optionalHeaderOffset = peOffset + 24;
  const buffer = Buffer.alloc(optionalHeaderOffset + 70);
  buffer.writeUInt16LE(0x5a4d, 0);
  buffer.writeUInt32LE(peOffset, 0x3c);
  buffer.writeUInt32LE(0x00004550, peOffset);
  buffer.writeUInt16LE(machine, peOffset + 4);
  buffer.writeUInt16LE(0xf0, peOffset + 20);
  buffer.writeUInt16LE(0x20b, optionalHeaderOffset);
  buffer.writeUInt16LE(subsystem, optionalHeaderOffset + 68);
  return buffer;
}

test("verifyDesktopPackage accepts macOS app with packaged sidecar", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-macos-package-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    const result = await verifyDesktopPackage({
      platform: "darwin",
      packagePath: join(root, "投研罗盘.app"),
    });

    assert.deepEqual(result, {
      platform: "darwin",
      desktopBinary: join(macosDir, "invest-compass-desktop"),
      sidecarBinary: join(macosDir, "invest-compas-core"),
    });
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app bundle name mismatching product name", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-macos-package-name-"));
  try {
    const appPath = join(root, "错误名称.app");
    const macosDir = join(appPath, "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(appPath, "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
        }),
      /macOS app bundle name mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app display name mismatching product name", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-macos-display-name-"));
  try {
    const appPath = join(root, "投研罗盘.app");
    const macosDir = join(appPath, "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(appPath, "Contents", "Info.plist"),
      infoPlistExecutableWithMetadata({
        executable: "invest-compass-desktop",
        packageType: "APPL",
        displayName: "错误名称",
        identifier: "com.lifei6671.investcompass",
        name: "投研罗盘",
        shortVersion: "0.1.0",
        bundleVersion: "0.1.0",
      }),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
        }),
      /macOS Info.plist CFBundleDisplayName mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app bundle name metadata mismatching product name", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-macos-bundle-name-"));
  try {
    const appPath = join(root, "投研罗盘.app");
    const macosDir = join(appPath, "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(appPath, "Contents", "Info.plist"),
      infoPlistExecutableWithMetadata({
        executable: "invest-compass-desktop",
        packageType: "APPL",
        displayName: "投研罗盘",
        identifier: "com.lifei6671.investcompass",
        name: "错误名称",
        shortVersion: "0.1.0",
        bundleVersion: "0.1.0",
      }),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
        }),
      /macOS Info.plist CFBundleName mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage validates macOS target architecture when requested", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-macos-target-package-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.doesNotReject(() =>
      verifyDesktopPackage({
        platform: "darwin",
        packagePath: join(root, "投研罗盘.app"),
        target: "aarch64-apple-darwin",
      }),
    );

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
          target: "x86_64-apple-darwin",
        }),
      /desktop binary architecture mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage accepts Windows package directory with sidecar exe", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-windows-package-"));
  try {
    await writeFile(join(root, "invest-compass-desktop.exe"), "desktop");
    await writeFile(join(root, "invest-compas-core.exe"), "sidecar");

    const result = await verifyDesktopPackage({
      platform: "win32",
      packagePath: root,
    });

    assert.deepEqual(result, {
      platform: "win32",
      desktopBinary: join(root, "invest-compass-desktop.exe"),
      sidecarBinary: join(root, "invest-compas-core.exe"),
    });
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects package root that is not a directory", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-file-package-root-"));
  try {
    const packageFile = join(root, "package-file");
    await writeFile(packageFile, "not a package directory");

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "win32",
          packagePath: packageFile,
        }),
      /desktop package root must be a directory/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage validates Windows target architecture when requested", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-windows-target-package-"));
  try {
    await writeFile(join(root, "invest-compass-desktop.exe"), peMachine(0x8664));
    await writeFile(join(root, "invest-compas-core.exe"), peMachine(0x8664));

    await assert.doesNotReject(() =>
      verifyDesktopPackage({
        platform: "win32",
        packagePath: root,
        target: "x86_64-pc-windows-msvc",
      }),
    );

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "win32",
          packagePath: root,
          target: "aarch64-apple-darwin",
        }),
      /target aarch64-apple-darwin does not match platform win32/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects Windows console subsystem when target is requested", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-windows-console-package-"));
  try {
    await writeFile(join(root, "invest-compass-desktop.exe"), peMachine(0x8664));
    await writeFile(join(root, "invest-compas-core.exe"), peMachine(0x8664, 3));

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "win32",
          packagePath: root,
          target: "x86_64-pc-windows-msvc",
        }),
      /packaged sidecar binary subsystem mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects package without sidecar", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-missing-sidecar-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
        }),
      /packaged sidecar binary is missing/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app without Info.plist executable metadata", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-missing-info-plist-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
        }),
      /macOS Info.plist is missing/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app with non-application package type", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-invalid-package-type-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutableWithPackageType("invest-compass-desktop", "FMWK"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
          target: "aarch64-apple-darwin",
        }),
      /macOS Info.plist CFBundlePackageType mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app without bundle identifier", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-missing-bundle-identifier-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutableWithMetadata({
        executable: "invest-compass-desktop",
        packageType: "APPL",
        displayName: "投研罗盘",
        identifier: "",
        name: "投研罗盘",
        shortVersion: "0.1.0",
        bundleVersion: "0.1.0",
      }),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
          target: "aarch64-apple-darwin",
        }),
      /macOS Info.plist missing CFBundleIdentifier/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app with mismatched bundle identifier", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-mismatched-bundle-identifier-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutableWithMetadata({
        executable: "invest-compass-desktop",
        packageType: "APPL",
        displayName: "投研罗盘",
        identifier: "com.example.unexpected",
        name: "投研罗盘",
        shortVersion: "0.1.0",
        bundleVersion: "0.1.0",
      }),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
          target: "aarch64-apple-darwin",
        }),
      /macOS Info.plist CFBundleIdentifier mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app with mismatched version metadata", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-mismatched-version-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutableWithMetadata({
        executable: "invest-compass-desktop",
        packageType: "APPL",
        displayName: "投研罗盘",
        identifier: "com.lifei6671.investcompass",
        name: "投研罗盘",
        shortVersion: "9.9.9",
        bundleVersion: "0.1.0",
      }),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
          target: "aarch64-apple-darwin",
        }),
      /macOS Info.plist CFBundleShortVersionString mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects symlinked macOS Info.plist", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-symlinked-info-plist-"));
  try {
    const appPath = join(root, "投研罗盘.app");
    const contentsDir = join(appPath, "Contents");
    const macosDir = join(contentsDir, "MacOS");
    const externalDir = join(root, "external-metadata");
    await mkdir(macosDir, { recursive: true });
    await mkdir(externalDir, { recursive: true });
    await writeFile(
      join(externalDir, "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await symlink(join(externalDir, "Info.plist"), join(contentsDir, "Info.plist"));
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
          target: "aarch64-apple-darwin",
        }),
      /macOS Info.plist must be packaged as a regular file/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS app with missing declared icon resource", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-missing-icon-resource-"));
  try {
    const appPath = join(root, "投研罗盘.app");
    const macosDir = join(appPath, "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(appPath, "Contents", "Info.plist"),
      infoPlistExecutableWithIcon("invest-compass-desktop", "icon.icns"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
          target: "aarch64-apple-darwin",
        }),
      /macOS icon resource is missing/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects symlinked macOS Resources directory", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-symlinked-resources-dir-"));
  try {
    const appPath = join(root, "投研罗盘.app");
    const contentsDir = join(appPath, "Contents");
    const macosDir = join(contentsDir, "MacOS");
    const externalResourcesDir = join(root, "external-resources");
    await mkdir(macosDir, { recursive: true });
    await mkdir(externalResourcesDir, { recursive: true });
    await writeFile(
      join(contentsDir, "Info.plist"),
      infoPlistExecutableWithIcon("invest-compass-desktop", "icon.icns"),
    );
    await writeFile(join(externalResourcesDir, "icon.icns"), "icon");
    await symlink(externalResourcesDir, join(contentsDir, "Resources"));
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
          target: "aarch64-apple-darwin",
        }),
      /macOS Resources directory must not be a symlink/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS icon resource paths escaping Resources", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-escaping-icon-resource-"));
  try {
    const appPath = join(root, "投研罗盘.app");
    const contentsDir = join(appPath, "Contents");
    const macosDir = join(contentsDir, "MacOS");
    const resourcesDir = join(contentsDir, "Resources");
    await mkdir(macosDir, { recursive: true });
    await mkdir(resourcesDir, { recursive: true });
    await writeFile(
      join(contentsDir, "Info.plist"),
      infoPlistExecutableWithIcon("invest-compass-desktop", "../MacOS/invest-compas-core"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
          target: "aarch64-apple-darwin",
        }),
      /macOS icon resource path must stay inside Contents\/Resources/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects empty packaged sidecar binary", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-empty-sidecar-"));
  try {
    await writeFile(join(root, "invest-compass-desktop.exe"), "desktop");
    await writeFile(join(root, "invest-compas-core.exe"), "");

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "win32",
          packagePath: root,
        }),
      /packaged sidecar binary is empty/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects empty desktop binary", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-empty-desktop-"));
  try {
    await writeFile(join(root, "invest-compass-desktop.exe"), "");
    await writeFile(join(root, "invest-compas-core.exe"), "sidecar");

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "win32",
          packagePath: root,
        }),
      /desktop binary is empty/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS package paths without app suffix", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-plain-macos-dir-"));
  try {
    const macosDir = join(root, "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: root,
        }),
      /macOS app bundle path must end with .app/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS binaries without executable permission", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-non-executable-macos-package-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
        }),
      /desktop binary is not executable/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects macOS sidecar without executable permission", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-non-executable-sidecar-package-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), "desktop");
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await writeFile(join(macosDir, "invest-compas-core"), "sidecar");

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
        }),
      /packaged sidecar binary is not executable/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects symlinked packaged binaries", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-symlinked-package-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    const externalDir = join(root, "external-bin");
    await mkdir(macosDir, { recursive: true });
    await mkdir(externalDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(externalDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(externalDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(externalDir, "invest-compass-desktop"), 0o755);
    await chmod(join(externalDir, "invest-compas-core"), 0o755);
    await symlink(
      join(externalDir, "invest-compass-desktop"),
      join(macosDir, "invest-compass-desktop"),
    );
    await symlink(
      join(externalDir, "invest-compas-core"),
      join(macosDir, "invest-compas-core"),
    );

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: join(root, "投研罗盘.app"),
          target: "aarch64-apple-darwin",
        }),
      /desktop binary must be packaged as a regular file/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects symlinked macOS executable directory", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-symlinked-macos-dir-"));
  try {
    const appPath = join(root, "投研罗盘.app");
    const contentsDir = join(appPath, "Contents");
    const externalMacosDir = join(root, "external-bin");
    await mkdir(contentsDir, { recursive: true });
    await mkdir(externalMacosDir, { recursive: true });
    await writeFile(
      join(contentsDir, "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(
      join(externalMacosDir, "invest-compass-desktop"),
      machO64CpuType(0x0100000c),
    );
    await writeFile(join(externalMacosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(externalMacosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(externalMacosDir, "invest-compas-core"), 0o755);
    await symlink(externalMacosDir, join(contentsDir, "MacOS"));

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: appPath,
          target: "aarch64-apple-darwin",
        }),
      /macOS executable directory must not be a symlink/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage rejects symlinked package roots", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-symlinked-package-root-"));
  try {
    const realAppPath = join(root, "real", "投研罗盘.app");
    const linkedAppPath = join(root, "投研罗盘.app");
    const macosDir = join(realAppPath, "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(realAppPath, "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);
    await symlink(realAppPath, linkedAppPath);

    await assert.rejects(
      () =>
        verifyDesktopPackage({
          platform: "darwin",
          packagePath: linkedAppPath,
          target: "aarch64-apple-darwin",
        }),
      /desktop package root must not be a symlink/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage CLI accepts pnpm forwarded argument separator", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-cli-package-"));
  try {
    const macosDir = join(root, "投研罗盘.app", "Contents", "MacOS");
    await mkdir(macosDir, { recursive: true });
    await writeFile(
      join(root, "投研罗盘.app", "Contents", "Info.plist"),
      infoPlistExecutable("invest-compass-desktop"),
    );
    await writeFile(join(macosDir, "invest-compass-desktop"), machO64CpuType(0x0100000c));
    await writeFile(join(macosDir, "invest-compas-core"), machO64CpuType(0x0100000c));
    await chmod(join(macosDir, "invest-compass-desktop"), 0o755);
    await chmod(join(macosDir, "invest-compas-core"), 0o755);

    const { stdout } = await execFileAsync(process.execPath, [
      new URL("./verify-desktop-package.mjs", import.meta.url).pathname,
      "--",
      "--platform=darwin",
      "--target=aarch64-apple-darwin",
      join(root, "投研罗盘.app"),
    ]);

    assert.match(stdout, /desktop binary:/);
    assert.match(stdout, /sidecar binary:/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifyDesktopPackage CLI rejects missing package path", async () => {
  await assert.rejects(
    () =>
      execFileAsync(process.execPath, [
        new URL("./verify-desktop-package.mjs", import.meta.url).pathname,
        "--platform=darwin",
      ]),
    (error) => {
      assert.equal(error.code, 1);
      assert.match(error.stderr, /packagePath is required/);
      return true;
    },
  );
});

test("verifyDesktopPackage CLI rejects platform option without value", async () => {
  await assert.rejects(
    () =>
      execFileAsync(process.execPath, [
        new URL("./verify-desktop-package.mjs", import.meta.url).pathname,
        "--platform",
      ]),
    (error) => {
      assert.equal(error.code, 1);
      assert.match(error.stderr, /platform is required/);
      return true;
    },
  );
});
