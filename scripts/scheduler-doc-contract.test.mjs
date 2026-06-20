import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const schedulerTypesSource = await readFile(
  new URL("../apps/sidecar-core/internal/service/scheduler/types.go", import.meta.url),
  "utf8",
);
const schedulerRegistrySource = await readFile(
  new URL("../apps/sidecar-core/internal/service/scheduler/registry.go", import.meta.url),
  "utf8",
);
const schedulerDesignDoc = await readFile(
  new URL("../docs/2026-06-19-invest-compass-scheduler-design.md", import.meta.url),
  "utf8",
);
const acceptanceReport = await readFile(
  new URL("../docs/2026-06-18-invest-compass-acceptance-report.md", import.meta.url),
  "utf8",
);

function extractGoTriggerValues(source) {
  return [...source.matchAll(/Trigger[A-Za-z]+\s+=\s+"([^"]+)"/g)]
    .map((match) => match[1])
    .sort();
}

function extractCronTypeConstants(source) {
  return new Map(
    [...source.matchAll(/(CronType[A-Za-z]+)\s+=\s+"([^"]+)"/g)]
      .map((match) => [match[1], match[2]]),
  );
}

function extractRegistryCronTypeValues(registrySource, constants) {
  return [...registrySource.matchAll(/\{CronType:\s+(CronType[A-Za-z]+)/g)]
    .map((match) => {
      const value = constants.get(match[1]);
      assert.ok(value, `DefaultJobRegistry 引用了未知 cron_type 常量 ${match[1]}`);
      return value;
    })
    .sort();
}

function extractDocumentTriggerValues(markdown) {
  const match = markdown.match(/`trigger_type` 取值：\n\n```text\n([\s\S]*?)\n```/);
  assert.ok(match, "调度方案必须保留 trigger_type 取值代码块");
  return match[1]
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .sort();
}

function extractDocumentCronTypeExamples(markdown) {
  return [...markdown.matchAll(/^cron_type:\s*(\S+)/gm)]
    .map((match) => match[1])
    .sort();
}

function extractKnownTriggerWords(text) {
  const constants = new Set(extractGoTriggerValues(schedulerTypesSource));
  const triggerLikeWords = new Set([...constants, "startup", "cron", "manual", "catchup"]);
  return [...triggerLikeWords]
    .filter((value) => new RegExp(`\\b${value}\\b`).test(text))
    .sort();
}

test("scheduler design trigger_type values match Go constants", () => {
  assert.deepEqual(
    extractDocumentTriggerValues(schedulerDesignDoc),
    extractGoTriggerValues(schedulerTypesSource),
    "调度方案里的 trigger_type 枚举必须和 Go scheduler 常量保持一致",
  );
});

test("scheduler design cron_type examples match default registry", () => {
  const constants = extractCronTypeConstants(schedulerTypesSource + "\n" + schedulerRegistrySource);

  assert.deepEqual(
    extractDocumentCronTypeExamples(schedulerDesignDoc),
    extractRegistryCronTypeValues(schedulerRegistrySource, constants),
    "调度方案里的默认 cron_type 示例必须和 DefaultJobRegistry 保持一致",
  );
});

test("acceptance report scheduler trigger names match Go constants", () => {
  const schedulerSection = acceptanceReport.match(/## 4\. 调度专题验收记录\n([\s\S]*?)\n## 5\./);
  assert.ok(schedulerSection, "验收报告必须保留调度专题验收记录章节");

  assert.deepEqual(
    extractKnownTriggerWords(schedulerSection[1]),
    extractGoTriggerValues(schedulerTypesSource),
    "验收报告调度章节不能继续使用已经不存在的 trigger_type 名称",
  );
});
