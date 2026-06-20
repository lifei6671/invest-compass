import test from "node:test";
import assert from "node:assert/strict";

import * as providerSmoke from "./provider-smoke.mjs";

const { buildProviderSmokePlan, readProviderSmokeOptions } = providerSmoke;

test("provider smoke defaults to dry run without network", () => {
  const options = readProviderSmokeOptions([]);
  const plan = buildProviderSmokePlan(options);

  assert.equal(options.allowNetwork, false);
  assert.equal(options.confirmProviderTerms, false);
  assert.equal(plan.networkEnabled, false);
  assert.equal(plan.checks.every((check) => check.status === "skipped"), true);
  assert.equal(
    plan.checks.every((check) => check.skipReason === "network smoke disabled"),
    true,
  );
});

test("provider smoke requires explicit provider terms confirmation before network", () => {
  const options = readProviderSmokeOptions(["--allow-network"]);

  assert.throws(
    () => buildProviderSmokePlan(options),
    /--confirm-provider-terms is required/,
  );
});

test("provider smoke plans market and news checks after explicit confirmation", () => {
  const options = readProviderSmokeOptions([
    "--",
    "--allow-network",
    "--confirm-provider-terms",
  ]);
  const plan = buildProviderSmokePlan(options);

  assert.equal(plan.networkEnabled, true);
  assert.deepEqual(
    plan.checks.map((check) => check.name),
    ["sina-suggest", "sina-quote", "tencent-kline", "eastmoney-kline", "sina-live-news"],
  );
  assert.equal(plan.checks.every((check) => check.status === "pending"), true);
  assert.equal(plan.checks.every((check) => check.url.startsWith("https://")), true);
});

test("provider smoke validates endpoint-specific response bodies", () => {
  assert.equal(typeof providerSmoke.validateProviderSmokeBody, "function");

  assert.equal(
    providerSmoke.validateProviderSmokeBody("sina-quote", 'var hq_str_sh600519="贵州茅台,1,2";'),
    true,
  );
  assert.equal(providerSmoke.validateProviderSmokeBody("sina-quote", "<html>ok</html>"), false);

  assert.equal(
    providerSmoke.validateProviderSmokeBody(
      "tencent-kline",
      '{"code":0,"data":{"sh600519":{"day":[["2026-06-19","1","2"]]}}}',
    ),
    true,
  );
  assert.equal(providerSmoke.validateProviderSmokeBody("tencent-kline", '{"code":0}'), false);

  assert.equal(
    providerSmoke.validateProviderSmokeBody(
      "eastmoney-kline",
      '{"data":{"code":"600519","klines":["2026-06-19,1,2,3,4"]}}',
    ),
    true,
  );
  assert.equal(providerSmoke.validateProviderSmokeBody("eastmoney-kline", '{"data":null}'), false);

  assert.equal(
    providerSmoke.validateProviderSmokeBody(
      "sina-live-news",
      'callback({"result":{"data":{"feed":{"list":[{"title":"market"}]}}}})',
    ),
    true,
  );
  assert.equal(providerSmoke.validateProviderSmokeBody("sina-live-news", "callback({})"), false);
});

test("provider smoke classifies HTTP and response body failures", () => {
  assert.equal(typeof providerSmoke.classifyProviderSmokeResult, "function");

  assert.deepEqual(
    providerSmoke.classifyProviderSmokeResult({
      checkName: "sina-quote",
      responseOk: false,
      statusCode: 403,
      body: 'var hq_str_sh600519="贵州茅台,1,2";',
    }),
    {
      status: "failed",
      failureReason: "http_status_403",
    },
  );

  assert.deepEqual(
    providerSmoke.classifyProviderSmokeResult({
      checkName: "sina-quote",
      responseOk: true,
      statusCode: 200,
      body: "<html>ok</html>",
    }),
    {
      status: "failed",
      failureReason: "body_shape_mismatch",
    },
  );

  assert.deepEqual(
    providerSmoke.classifyProviderSmokeResult({
      checkName: "sina-quote",
      responseOk: true,
      statusCode: 200,
      body: 'var hq_str_sh600519="贵州茅台,1,2";',
    }),
    {
      status: "passed",
      failureReason: "",
    },
  );
});

test("provider smoke run uses injectable fetch and preserves failure reasons", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => {
    throw new Error("global fetch must not be used");
  };
  try {
    const result = await providerSmoke.runProviderSmoke({
      allowNetwork: true,
      confirmProviderTerms: true,
      fetchImpl: async (url) => {
        if (url.includes("suggest3.sinajs.cn")) {
          return {
            ok: false,
            status: 429,
            text: async () => "rate limited",
          };
        }
        return {
          ok: true,
          status: 200,
          text: async () => "<html>blocked</html>",
        };
      },
    });

    assert.equal(result.networkEnabled, true);
    assert.deepEqual(
      result.checks.map((check) => [check.name, check.status, check.failureReason]),
      [
        ["sina-suggest", "failed", "http_status_429"],
        ["sina-quote", "failed", "body_shape_mismatch"],
        ["tencent-kline", "failed", "body_shape_mismatch"],
        ["eastmoney-kline", "failed", "body_shape_mismatch"],
        ["sina-live-news", "failed", "body_shape_mismatch"],
      ],
    );
  } finally {
    globalThis.fetch = originalFetch;
  }
});
