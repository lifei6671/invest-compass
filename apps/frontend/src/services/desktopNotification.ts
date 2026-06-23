import { isPermissionGranted, requestPermission, sendNotification } from "@tauri-apps/plugin-notification";

export type DesktopNotificationPayload = {
  title: string;
  body: string;
};

export type DesktopNotificationKind = "task_success" | "task_failed" | "provider_error";

export type ConfiguredDesktopNotificationPayload = DesktopNotificationPayload & {
  kind: DesktopNotificationKind;
};

export type DesktopNotificationSettings = {
  systemEnabled: boolean;
  taskSuccessNotification: boolean;
  taskFailedNotification: boolean;
  providerErrorNotification: boolean;
};

export type DesktopNotificationResult =
  | { sent: true }
  | { sent: false; reason: "disabled" | "permission_denied" | "failed" };

export async function sendConfiguredDesktopNotification(
  payload: ConfiguredDesktopNotificationPayload,
  settings: DesktopNotificationSettings,
): Promise<DesktopNotificationResult> {
  if (!settings.systemEnabled || !isNotificationKindEnabled(payload.kind, settings)) {
    return { sent: false, reason: "disabled" };
  }
  return sendDesktopNotification(payload);
}

export async function sendDesktopNotification(payload: DesktopNotificationPayload): Promise<DesktopNotificationResult> {
  try {
    let granted = await isPermissionGranted();
    if (!granted) {
      granted = (await requestPermission()) === "granted";
    }
    if (!granted) {
      return { sent: false, reason: "permission_denied" };
    }
    sendNotification({
      title: redactNotificationText(payload.title),
      body: redactNotificationText(payload.body),
    });
    return { sent: true };
  } catch {
    return { sent: false, reason: "failed" };
  }
}

function isNotificationKindEnabled(kind: DesktopNotificationKind, settings: DesktopNotificationSettings): boolean {
  if (kind === "task_success") {
    return settings.taskSuccessNotification;
  }
  if (kind === "task_failed") {
    return settings.taskFailedNotification;
  }
  return settings.providerErrorNotification;
}

function redactNotificationText(value: string): string {
  return value
    .replace(/(Proxy-Authorization\s*:\s*(?:Bearer\s+)?)[^\s;]+/gi, "$1***")
    .replace(/(Authorization\s*:\s*(?:Bearer\s+)?)[^\s;]+/gi, "$1***")
    .replace(/(API\s*Key\s*)[A-Za-z0-9._-]+/gi, "$1***")
    .replace(/(token=)[^;\s]+/gi, "$1***")
    .replace(/sk-[A-Za-z0-9._-]+/g, "sk-***");
}
