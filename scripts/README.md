# Scripts

This directory is reserved for workspace automation such as sidecar packaging,
desktop build orchestration, and release checks. Executable scripts will be
added only when their corresponding checklist task defines the behavior.

## Local release candidate check

Use `pnpm --dir apps release:check:local` on an Apple Silicon macOS machine to
rerun the local acceptance baseline, rehearse SQLite backup restore and
remigration, rebuild and verify every first-version sidecar target, rebuild the
app, produce the macOS `.app`, verify the generated package structure, run
packaged Go core handshake smoke, and run `git diff --check` from the repository
root. This command does not run network Provider smoke and does not replace
macOS Intel, Windows, or manual desktop capability acceptance.
Go test and sidecar build steps run with `-mod=readonly`, so dependency file
updates must be reviewed explicitly instead of being produced by release checks.

Use `pnpm --dir apps sqlite:upgrade-rehearsal` to repeat the focused SQLite
upgrade rehearsal without rebuilding the desktop app. The command runs the DAO
test that creates a user database, backs it up before migration, restores the
backup to a fresh SQLite file, reruns migration, and verifies that existing
settings data survived. It is an automated pre-release check, not a substitute
for upgrading a real installed user profile after packaging.

Use `pnpm --dir apps sidecar:smoke` after producing the Apple Silicon macOS
`.app` to start the packaged `invest-compas-core`, send the stdin runtime token
handshake, call `/internal/health`, and close it through `/internal/shutdown`.

## Sidecar target verification

Use `pnpm --dir apps sidecar:check-targets` before cross-platform packaging to
rebuild and verify every first-version sidecar target:

```bash
pnpm --dir apps sidecar:check-targets
```

The command builds `aarch64-apple-darwin`, `x86_64-apple-darwin`, and
`x86_64-pc-windows-msvc` sidecar binaries, then verifies that each artifact is a
real non-empty file rather than a symlink. macOS targets must have executable
permission and matching Mach-O CPU type. The Windows target must be PE x86_64
and use the GUI subsystem so launching the packaged desktop app does not open a
console window for the sidecar. Because the sidecar uses the SQLite cgo driver,
the build script forces `CGO_ENABLED=1`, and target verification rejects Go
builds that still carry `CGO_ENABLED=0` metadata.

Use `pnpm --dir apps sidecar:verify-targets` when the binaries were already
built and only the artifact checks need to be repeated. These commands do not
replace real macOS Intel or Windows startup, vault, tray, notification, or
autostart acceptance.

## Desktop package verification

Use `verify-desktop-package.mjs` after producing a desktop package or unpacked
Windows directory:

```bash
pnpm --dir apps package:verify -- --platform=darwin "desktop/src-tauri/target/release/bundle/macos/投研罗盘.app"
pnpm --dir apps package:verify -- --platform=darwin --target=aarch64-apple-darwin "desktop/src-tauri/target/release/bundle/macos/投研罗盘.app"
pnpm --dir apps package:verify -- --platform=win32 "path/to/unpacked/windows/package"
pnpm --dir apps package:verify -- --platform=win32 --target=x86_64-pc-windows-msvc "path/to/unpacked/windows/package"
```

The script only checks package structure: the desktop package root and macOS
`Contents` / `Contents/MacOS` paths must be real directories rather than
symlinks, macOS `Contents/Resources` must also be a real directory when an icon
is declared, and macOS `Info.plist`, desktop binary, and packaged sidecar binary
must be real non-empty files rather than symlinks.
macOS verification requires a `.app` bundle path whose name matches the Tauri
config `productName`, executable bits on the packaged binaries, and an
`Info.plist` whose `CFBundleExecutable` points to the packaged desktop binary,
whose `CFBundlePackageType` is `APPL`, and whose `CFBundleDisplayName`,
`CFBundleName`, `CFBundleIdentifier`, `CFBundleShortVersionString`, and
`CFBundleVersion` match the Tauri config.
When `Info.plist` declares `CFBundleIconFile`, the referenced file must be a
direct `Contents/Resources` child and exist as a real non-empty file;
Windows verification expects an unpacked package directory. Pass
`--target=<triple>` to also verify the binary architecture for `aarch64-apple-darwin`,
`x86_64-apple-darwin`, or `x86_64-pc-windows-msvc`; Windows target verification
also rejects console-subsystem binaries because packaged desktop and sidecar
executables must be GUI-subsystem PE files. The CLI fails early when the package
path is missing or `--platform` / `--target` has no value. It does not replace
real macOS / Windows startup, vault, tray, notification, or autostart acceptance.

## Provider smoke verification

Use `provider-smoke.mjs` to prepare or run live Market / News Provider smoke
checks:

```bash
pnpm --dir apps provider:smoke
pnpm --dir apps provider:smoke -- --allow-network --confirm-provider-terms
```

The default command is a dry run and does not access the network. Live smoke
checks require both `--allow-network` and `--confirm-provider-terms` so the
operator explicitly confirms data source terms, authorization, and rate-limit
boundaries before touching third-party endpoints. Live checks require each
endpoint to return the expected provider-specific body shape, not just a non-empty
HTTP 200 response; this still does not prove data source authorization,
redistribution rights, or production availability.

Live failure output includes a stable `failureReason` so operators can
distinguish HTTP rejection (`http_status_<code>`), provider body shape mismatch
(`body_shape_mismatch`), and request transport errors (`request_error`).
