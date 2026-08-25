import { readdir, writeFile } from "node:fs/promises";
import { join, relative, sep } from "node:path";

function escapeXml(value) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&apos;");
}

async function collectHtmlFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const entryPath = join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await collectHtmlFiles(entryPath)));
    } else if (entry.isFile() && entry.name.endsWith(".html")) {
      files.push(entryPath);
    }
  }

  return files;
}

function routeFromHtmlFile(outDir, filePath) {
  const outputPath = relative(outDir, filePath).split(sep).join("/");

  if (outputPath === "index.html" || outputPath === "404.html") {
    return undefined;
  }

  const route = outputPath.endsWith("/index.html")
    ? outputPath.slice(0, -"/index.html".length)
    : outputPath.slice(0, -".html".length);

  if (!route || route.split("/").some((segment) => segment.startsWith("__docusaurus"))) {
    return undefined;
  }

  return route;
}

export async function generateSitemap({ baseUrl, outDir, siteUrl }) {
  const routes = (await collectHtmlFiles(outDir))
    .map((filePath) => routeFromHtmlFile(outDir, filePath))
    .filter(Boolean)
    .sort((left, right) => left.localeCompare(right, "en"));

  if (routes.length === 0) {
    throw new Error("Cannot generate a sitemap without documentation routes.");
  }

  const normalizedBaseUrl = baseUrl === "/" ? "/" : `/${baseUrl.replace(/^\/+|\/+$/g, "")}/`;
  const normalizedSiteUrl = siteUrl.replace(/\/$/, "");
  const entries = routes.map((route) => {
    const location = `${normalizedSiteUrl}${normalizedBaseUrl}${encodeURI(route)}`;
    return `  <url><loc>${escapeXml(location)}</loc></url>`;
  });
  const sitemap = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...entries,
    "</urlset>",
    "",
  ].join("\n");

  await writeFile(join(outDir, "sitemap.xml"), sitemap, "utf8");
}

if (import.meta.main) {
  const { default: siteConfig } = await import("../docusaurus.config.ts");
  await generateSitemap({
    baseUrl: siteConfig.baseUrl,
    outDir: join(import.meta.dirname, "../build"),
    siteUrl: siteConfig.url,
  });
}
