import type { CredentialHealthItem, CredentialStatus, CredentialTestResult } from "../types";

type StatusKind = CredentialStatus | CredentialHealthItem["status"] | CredentialTestResult["status"] | "success";

export function credentialStatusText(status: StatusKind) {
  switch (status) {
    case "normal":
      return "正常";
    case "not_configured":
      return "未配置";
    case "expired":
      return "已过期";
    case "expiring":
      return "即将过期";
    case "failed":
      return "异常";
    case "limited":
      return "受限";
    case "success":
      return "成功";
    case "untested":
      return "未测试";
    default:
      return "连接成功";
  }
}

export function CredentialStatusTag(props: { status: StatusKind; text?: string }) {
  const tone = props.status === "normal" || props.status === "success" ? "ok" : props.status === "failed" || props.status === "expired" ? "failed" : props.status === "untested" ? "muted" : "warn";
  return <span className={["credential-tag", `credential-tag-${tone}`].join(" ")}>{props.text ?? credentialStatusText(props.status)}</span>;
}

export function authTypeText(authType: string) {
  switch (authType) {
    case "none":
      return "无需凭据";
    case "api_key":
      return "API Key";
    case "cookie":
      return "Cookie";
    case "bearer_token":
      return "Bearer Token";
    default:
      return "Custom Header";
  }
}
