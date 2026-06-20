import { isPermissionGranted, requestPermission, sendNotification } from "@tauri-apps/plugin-notification";

export type DesktopNotificationPayload = {
  title: string;
  body: string;
};

export async function sendDesktopNotification(payload: DesktopNotificationPayload): Promise<void> {
  let granted = await isPermissionGranted();
  if (!granted) {
    granted = (await requestPermission()) === "granted";
  }
  if (granted) {
    sendNotification(payload);
  }
}
