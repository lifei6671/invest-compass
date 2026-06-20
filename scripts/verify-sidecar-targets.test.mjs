import test from "node:test";
import assert from "node:assert/strict";
import { chmod, mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";

import { verifySidecarTargets } from "./verify-sidecar-targets.mjs";

function machO64CpuType(cpuType) {
  const buffer = Buffer.alloc(8);
  buffer.writeUInt32LE(0xfeedfacf, 0);
  buffer.writeInt32LE(cpuType, 4);
  return buffer;
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

async function writeSidecarTargets(binariesDir, overrides = {}) {
  await mkdir(binariesDir, { recursive: true });
  const files = {
    "invest-compas-core-aarch64-apple-darwin": machO64CpuType(0x0100000c),
    "invest-compas-core-x86_64-apple-darwin": machO64CpuType(0x01000007),
    "invest-compas-core-x86_64-pc-windows-msvc.exe": peMachine(0x8664),
    ...overrides,
  };
  for (const [fileName, content] of Object.entries(files)) {
    const path = join(binariesDir, fileName);
    await writeFile(path, content);
    if (!fileName.endsWith(".exe")) {
      await chmod(path, 0o755);
    }
  }
}

test("verifySidecarTargets accepts all first-version sidecar binaries", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-sidecar-targets-"));
  try {
    const binariesDir = join(root, "binaries");
    await writeSidecarTargets(binariesDir);

    const result = await verifySidecarTargets({ binariesDir });

    assert.deepEqual(
      result.map((item) => item.target),
      ["aarch64-apple-darwin", "x86_64-apple-darwin", "x86_64-pc-windows-msvc"],
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifySidecarTargets rejects Windows console subsystem binaries", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-sidecar-console-"));
  try {
    const binariesDir = join(root, "binaries");
    await writeSidecarTargets(binariesDir, {
      "invest-compas-core-x86_64-pc-windows-msvc.exe": peMachine(0x8664, 3),
    });

    await assert.rejects(
      () => verifySidecarTargets({ binariesDir }),
      /sidecar target x86_64-pc-windows-msvc subsystem mismatch/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifySidecarTargets rejects Windows sidecars built without CGO", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-sidecar-cgo-"));
  try {
    const binariesDir = join(root, "binaries");
    await writeSidecarTargets(binariesDir, {
      "invest-compas-core-x86_64-pc-windows-msvc.exe": Buffer.concat([
        peMachine(0x8664),
        Buffer.from("\nbuild\tCGO_ENABLED=0\n", "utf8"),
      ]),
    });

    await assert.rejects(
      () => verifySidecarTargets({ binariesDir }),
      /sidecar target x86_64-pc-windows-msvc was built with CGO_ENABLED=0/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("verifySidecarTargets rejects missing target binaries", async () => {
  const root = await mkdtemp(join(tmpdir(), "invest-compass-sidecar-missing-"));
  try {
    const binariesDir = join(root, "binaries");
    await writeSidecarTargets(binariesDir);
    await rm(join(binariesDir, "invest-compas-core-x86_64-apple-darwin"), { force: true });

    await assert.rejects(
      () => verifySidecarTargets({ binariesDir }),
      /sidecar target binary is missing/,
    );
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
