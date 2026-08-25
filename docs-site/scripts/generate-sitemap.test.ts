import { afterEach, expect, test } from "bun:test";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";

import { generateSitemap } from "./generate-sitemap.mjs";

const temporaryDirectories: string[] = [];

async function createBuildDirectory(files: string[]): Promise<string> {
  const buildDirectory = await mkdtemp(join(tmpdir(), "molii-docs-sitemap-"));
  temporaryDirectories.push(buildDirectory);

  for (const file of files) {
    const filePath = join(buildDirectory, file);
    await mkdir(join(filePath, ".."), { recursive: true });
    await writeFile(filePath, "<!doctype html>", "utf8");
  }

  return buildDirectory;
}

afterEach(async () => {
  await Promise.all(
    temporaryDirectories
      .splice(0)
      .map((directory) => rm(directory, { force: true, recursive: true })),
  );
});

test("generates a stable Development sitemap from built documentation pages", async () => {
  const buildDirectory = await createBuildDirectory([
    "index.html",
    "404.html",
    "quick-start/index.html",
    "api-reference/index.html",
    "providers/research&labs/index.html",
    "providers/x-ai/grok-imagine-image/index.html",
    "assets/runtime.js",
  ]);

  await generateSitemap({
    baseUrl: "/docs/",
    outDir: buildDirectory,
    siteUrl: "https://dev.molii.co",
  });

  const sitemap = await Bun.file(join(buildDirectory, "sitemap.xml")).text();

  expect(sitemap).toBe(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://dev.molii.co/docs/api-reference</loc></url>
  <url><loc>https://dev.molii.co/docs/providers/research&amp;labs</loc></url>
  <url><loc>https://dev.molii.co/docs/providers/x-ai/grok-imagine-image</loc></url>
  <url><loc>https://dev.molii.co/docs/quick-start</loc></url>
</urlset>
`);
  expect(sitemap).not.toContain("404");
  expect(sitemap).not.toContain("<loc>https://dev.molii.co/docs/</loc>");
});

test("uses the Production origin and root base path without changing route order", async () => {
  const buildDirectory = await createBuildDirectory(["z-last/index.html", "a-first/index.html"]);

  await generateSitemap({
    baseUrl: "/",
    outDir: buildDirectory,
    siteUrl: "https://molii.co",
  });

  const sitemap = await Bun.file(join(buildDirectory, "sitemap.xml")).text();
  expect(sitemap.indexOf("https://molii.co/a-first")).toBeLessThan(
    sitemap.indexOf("https://molii.co/z-last"),
  );
  expect(sitemap).not.toContain("https://molii.co//");
});
