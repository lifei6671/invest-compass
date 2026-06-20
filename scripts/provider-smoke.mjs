import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const providerChecks = [
  {
    name: "sina-suggest",
    url: "https://suggest3.sinajs.cn/suggest/?key=600519&name=suggestdata",
  },
  {
    name: "sina-quote",
    url: "https://hq.sinajs.cn/?list=sh600519",
    headers: {
      Referer: "https://finance.sina.com.cn/",
    },
  },
  {
    name: "tencent-kline",
    url: "https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=sh600519,day,,,5,",
  },
  {
    name: "eastmoney-kline",
    url: "https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=1.600519&klt=101&fqt=0&lmt=5&end=20500101&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57",
    headers: {
      Referer: "https://quote.eastmoney.com/",
    },
  },
  {
    name: "sina-live-news",
    url: "https://zhibo.sina.com.cn/api/zhibo/feed?callback=callback&page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20&type=0",
    headers: {
      Referer: "https://finance.sina.com.cn",
    },
  },
];

export function readProviderSmokeOptions(argv) {
  const options = {
    allowNetwork: false,
    confirmProviderTerms: false,
  };
  for (const arg of argv) {
    if (arg === "--") {
      continue;
    }
    if (arg === "--allow-network") {
      options.allowNetwork = true;
      continue;
    }
    if (arg === "--confirm-provider-terms") {
      options.confirmProviderTerms = true;
      continue;
    }
    throw new Error(`unexpected argument: ${arg}`);
  }
  return options;
}

export function buildProviderSmokePlan(options = {}) {
  const allowNetwork = options.allowNetwork === true;
  const confirmProviderTerms = options.confirmProviderTerms === true;
  if (allowNetwork && !confirmProviderTerms) {
    throw new Error("--confirm-provider-terms is required when --allow-network is set");
  }
  return {
    networkEnabled: allowNetwork,
    checks: providerChecks.map((check) => ({
      ...check,
      status: allowNetwork ? "pending" : "skipped",
      skipReason: allowNetwork ? "" : "network smoke disabled",
    })),
  };
}

export function validateProviderSmokeBody(checkName, body) {
  if (typeof body !== "string" || body.trim() === "") {
    return false;
  }
  switch (checkName) {
    case "sina-suggest":
      return /suggestdata[_\d]*\s*=/.test(body) && /600519/.test(body);
    case "sina-quote":
      return /var\s+hq_str_sh600519\s*=/.test(body) && body.includes(",");
    case "tencent-kline":
      return isTencentKlineBody(body);
    case "eastmoney-kline":
      return isEastMoneyKlineBody(body);
    case "sina-live-news":
      return /callback\s*\(/.test(body) && /"feed"|"list"|"title"/.test(body);
    default:
      return false;
  }
}

export function classifyProviderSmokeResult({ checkName, responseOk, statusCode, body }) {
  if (!responseOk) {
    return {
      status: "failed",
      failureReason: `http_status_${statusCode || "unknown"}`,
    };
  }
  if (!validateProviderSmokeBody(checkName, body)) {
    return {
      status: "failed",
      failureReason: "body_shape_mismatch",
    };
  }
  return {
    status: "passed",
    failureReason: "",
  };
}

export async function runProviderSmoke(options = {}) {
  const plan = buildProviderSmokePlan(options);
  if (!plan.networkEnabled) {
    return plan;
  }
  const fetchImpl = options.fetchImpl ?? fetch;
  const checks = [];
  for (const check of plan.checks) {
    checks.push(await runCheck(check, fetchImpl));
  }
  return {
    ...plan,
    checks,
  };
}

async function runCheck(check, fetchImpl) {
  try {
    const response = await fetchImpl(check.url, {
      headers: check.headers ?? {},
      signal: AbortSignal.timeout(10000),
    });
    const body = await response.text();
    const result = classifyProviderSmokeResult({
      checkName: check.name,
      responseOk: response.ok,
      statusCode: response.status,
      body,
    });
    return {
      ...check,
      ...result,
      statusCode: response.status,
      bodyBytes: Buffer.byteLength(body),
    };
  } catch (error) {
    return {
      ...check,
      status: "failed",
      failureReason: "request_error",
      error: error instanceof Error ? error.message : String(error),
    };
  }
}

function isTencentKlineBody(body) {
  try {
    const payload = JSON.parse(body);
    const rows = payload?.data?.sh600519?.day;
    return payload?.code === 0 && Array.isArray(rows) && rows.length > 0;
  } catch {
    return false;
  }
}

function isEastMoneyKlineBody(body) {
  try {
    const payload = JSON.parse(body);
    const rows = payload?.data?.klines;
    return payload?.data?.code === "600519" && Array.isArray(rows) && rows.length > 0;
  } catch {
    return false;
  }
}

async function main() {
  try {
    const result = await runProviderSmoke(readProviderSmokeOptions(process.argv.slice(2)));
    console.log(JSON.stringify(result, null, 2));
    if (result.checks.some((check) => check.status === "failed")) {
      process.exit(1);
    }
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

if (resolve(process.argv[1] ?? "") === fileURLToPath(import.meta.url)) {
  await main();
}
