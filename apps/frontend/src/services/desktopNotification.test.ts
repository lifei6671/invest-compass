import { describe, expect, test, vi, beforeEach } from "vitest";
import {
  sendConfiguredDesktopNotification,
  sendDesktopNotification,
  type DesktopNotificationSettings,
} from "./desktopNotification";

const isPermissionGrantedMock = vi.hoisted(() => vi.fn());
const requestPermissionMock = vi.hoisted(() => vi.fn());
const sendNotificationMock = vi.hoisted(() => vi.fn());

vi.mock("@tauri-apps/plugin-notification", () => ({
  isPermissionGranted: isPermissionGrantedMock,
  requestPermission: requestPermissionMock,
  sendNotification: sendNotificationMock,
}));

const enabledSettings: DesktopNotificationSettings = {
  systemEnabled: true,
  taskSuccessNotification: true,
  taskFailedNotification: true,
  providerErrorNotification: true,
};

beforeEach(() => {
  isPermissionGrantedMock.mockReset();
  requestPermissionMock.mockReset();
  sendNotificationMock.mockReset();
});

describe("sendConfiguredDesktopNotification", () => {
  test("系统级通知关闭时不请求权限也不发送系统通知", async () => {
    const result = await sendConfiguredDesktopNotification(
      { kind: "task_success", title: "任务完成", body: "分析报告已生成" },
      { ...enabledSettings, systemEnabled: false },
    );

    expect(result).toEqual({ sent: false, reason: "disabled" });
    expect(isPermissionGrantedMock).not.toHaveBeenCalled();
    expect(requestPermissionMock).not.toHaveBeenCalled();
    expect(sendNotificationMock).not.toHaveBeenCalled();
  });

  test("权限被拒绝时不抛错且只保留应用内通知链路", async () => {
    isPermissionGrantedMock.mockResolvedValue(false);
    requestPermissionMock.mockResolvedValue("denied");

    const result = await sendConfiguredDesktopNotification(
      { kind: "provider_error", title: "Provider 异常", body: "行情数据源不可用" },
      enabledSettings,
    );

    expect(result).toEqual({ sent: false, reason: "permission_denied" });
    expect(sendNotificationMock).not.toHaveBeenCalled();
  });

  test("发送前会脱敏正文中的敏感凭据", async () => {
    isPermissionGrantedMock.mockResolvedValue(true);

    const result = await sendConfiguredDesktopNotification(
      {
        kind: "task_failed",
        title: "任务失败",
        body: "Authorization: Bearer sk-live-123 Cookie: uid=1; token=abc API Key sk-real-456",
      },
      enabledSettings,
    );

    expect(result).toEqual({ sent: true });
    expect(sendNotificationMock).toHaveBeenCalledWith({
      title: "任务失败",
      body: expect.not.stringContaining("sk-live-123"),
    });
    expect(sendNotificationMock).toHaveBeenCalledWith({
      title: "任务失败",
      body: expect.not.stringContaining("token=abc"),
    });
    expect(sendNotificationMock).toHaveBeenCalledWith({
      title: "任务失败",
      body: expect.stringContaining("***"),
    });
  });
});

describe("sendDesktopNotification", () => {
  test("基础发送接口在权限拒绝时返回未发送而不是抛错", async () => {
    isPermissionGrantedMock.mockResolvedValue(false);
    requestPermissionMock.mockResolvedValue("denied");

    await expect(sendDesktopNotification({ title: "任务完成", body: "完成" })).resolves.toEqual({
      sent: false,
      reason: "permission_denied",
    });
  });
});
