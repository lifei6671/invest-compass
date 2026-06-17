import { readdirSync, readFileSync } from 'node:fs'
import path from 'node:path'
import { test } from 'node:test'
import assert from 'node:assert/strict'

const repositoryRoot = path.resolve(import.meta.dirname, '../../..')
const sidecarCoreRoot = path.join(repositoryRoot, 'apps/sidecar-core')
const rustRoot = path.join(repositoryRoot, 'apps/desktop/src-tauri')
const chinesePattern = /[\u4e00-\u9fff]/

test('Go 包都提供中文 doc.go 注释', () => {
  const failures = findGoPackageDocFailures(sidecarCoreRoot)

  assert.deepEqual(failures, [])
})

test('Go 和 Rust 函数都有中文注释', () => {
  const failures = [
    ...findGoFunctionCommentFailures(sidecarCoreRoot),
    ...findRustFunctionCommentFailures(rustRoot),
  ]

  assert.deepEqual(failures, [])
})

test('注释契约检查能识别缺少中文注释的函数', () => {
  const failures = findFunctionCommentFailures('sample.go', [
    'package sample',
    '',
    '// EnglishOnly documents nothing for local readers.',
    'func EnglishOnly() {}',
    '',
    'func MissingComment() {}',
    '',
    '// Valid 有中文注释。',
    'func Valid() {}',
  ])

  assert.deepEqual(failures, [
    'sample.go:4 EnglishOnly 缺少中文函数注释',
    'sample.go:6 MissingComment 缺少中文函数注释',
  ])
})

function findGoPackageDocFailures(root) {
  return walkDirectories(root)
    .filter((directory) => directoryHasGoPackage(directory))
    .flatMap((directory) => {
      const docPath = path.join(directory, 'doc.go')
      const packageName = packageNameForDirectory(directory)
      const relativePath = relative(docPath)

      try {
        const docText = readFileSync(docPath, 'utf8')
        const expectedPrefix = `// Package ${packageName} `
        if (!docText.includes(expectedPrefix) || !chinesePattern.test(docText)) {
          return [`${relativePath} 缺少中文 package 注释`]
        }
        return []
      } catch {
        return [`${relativePath} 缺少 doc.go`]
      }
    })
}

function findGoFunctionCommentFailures(root) {
  return walkFiles(root)
    .filter((filePath) => filePath.endsWith('.go'))
    .flatMap((filePath) =>
      findFunctionCommentFailures(relative(filePath), readLines(filePath)),
    )
}

function findRustFunctionCommentFailures(root) {
  return [
    path.join(root, 'build.rs'),
    ...walkFiles(path.join(root, 'src')).filter((filePath) => filePath.endsWith('.rs')),
  ].flatMap((filePath) =>
    findFunctionCommentFailures(relative(filePath), readLines(filePath)),
  )
}

function findFunctionCommentFailures(relativePath, lines) {
  const failures = []

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index].trim()
    const functionName = goFunctionName(line) ?? rustFunctionName(line)
    if (!functionName) {
      continue
    }

    const comment = nearestLeadingComment(lines, index)
    if (!chinesePattern.test(comment)) {
      failures.push(`${relativePath}:${index + 1} ${functionName} 缺少中文函数注释`)
    }
  }

  return failures
}

function nearestLeadingComment(lines, functionIndex) {
  let index = functionIndex - 1
  const comments = []

  while (index >= 0) {
    const trimmed = lines[index].trim()
    if (trimmed === '' || trimmed.startsWith('#[')) {
      index -= 1
      continue
    }
    if (trimmed.startsWith('//')) {
      comments.unshift(trimmed)
      index -= 1
      continue
    }
    break
  }

  return comments.join('\n')
}

function goFunctionName(line) {
  const named = line.match(/^func\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/)
  if (named) {
    return named[1]
  }

  const method = line.match(/^func\s+\([^)]*\)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/)
  return method?.[1] ?? null
}

function rustFunctionName(line) {
  const matched = line.match(/^(?:pub\s+)?(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(/)
  return matched?.[1] ?? null
}

function walkDirectories(root) {
  const directories = [root]
  for (const item of readdirSync(root, { withFileTypes: true })) {
    if (item.isDirectory()) {
      directories.push(...walkDirectories(path.join(root, item.name)))
    }
  }
  return directories
}

function walkFiles(root) {
  const files = []
  for (const item of readdirSync(root, { withFileTypes: true })) {
    const itemPath = path.join(root, item.name)
    if (item.isDirectory()) {
      files.push(...walkFiles(itemPath))
    } else {
      files.push(itemPath)
    }
  }
  return files
}

function directoryHasGoPackage(directory) {
  return readdirSync(directory, { withFileTypes: true }).some(
    (item) => item.isFile() && item.name.endsWith('.go'),
  )
}

function packageNameForDirectory(directory) {
  const firstGoFile = readdirSync(directory)
    .filter((fileName) => fileName.endsWith('.go'))
    .sort()[0]
  const packageLine = readLines(path.join(directory, firstGoFile)).find((line) =>
    line.startsWith('package '),
  )

  return packageLine.replace('package ', '').trim()
}

function readLines(filePath) {
  return readFileSync(filePath, 'utf8').split(/\r?\n/)
}

function relative(filePath) {
  return path.relative(repositoryRoot, filePath)
}
