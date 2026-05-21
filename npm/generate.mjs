import { readFileSync, writeFileSync, mkdirSync, copyFileSync, chmodSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = dirname(here);
const outDir = join(here, "dist");
const version = process.argv[2]?.replace(/^v/, "");

if (!version) {
  console.error("usage: generate.mjs <version>");
  process.exit(1);
}

const targets = [
  { goos: "windows", goarch: "amd64", platform: "win32-x64",    os: "win32",  cpu: "x64",   binary: "preflight.exe" },
  { goos: "darwin",  goarch: "arm64", platform: "darwin-arm64", os: "darwin", cpu: "arm64", binary: "preflight" },
  { goos: "darwin",  goarch: "amd64", platform: "darwin-x64",   os: "darwin", cpu: "x64",   binary: "preflight" },
  { goos: "linux",   goarch: "amd64", platform: "linux-x64",    os: "linux",  cpu: "x64",   binary: "preflight" },
  { goos: "linux",   goarch: "arm64", platform: "linux-arm64",  os: "linux",  cpu: "arm64", binary: "preflight" },
];

const artifacts = JSON.parse(readFileSync(join(root, "dist", "artifacts.json"), "utf8"));
const platformTmpl = readFileSync(join(here, "package.platform.json.tmpl"), "utf8");
const mainTmpl = readFileSync(join(here, "package.main.json.tmpl"), "utf8");
const wrapper = readFileSync(join(here, "wrapper.js"), "utf8");

const fill = (tmpl, vars) =>
  Object.entries(vars).reduce((acc, [k, v]) => acc.replaceAll(`{{${k}}}`, v), tmpl);

for (const target of targets) {
  const binArtifact = artifacts.find(
    (a) => a.type === "Binary" && a.goos === target.goos && a.goarch === target.goarch &&
           (target.goarch !== "amd64" || a.goamd64 === "v1")
  );

  if (!binArtifact) throw new Error(`no binary for ${target.goos}/${target.goarch}`);

  const name = `@jacobjoergensen/preflight-${target.platform}`;
  const pkgDir = join(outDir, `preflight-${target.platform}`);

  mkdirSync(join(pkgDir, "bin"), { recursive: true });
  copyFileSync(join(root, binArtifact.path), join(pkgDir, "bin", target.binary));
  chmodSync(join(pkgDir, "bin", target.binary), 0o755);

  writeFileSync(
    join(pkgDir, "package.json"),
    fill(platformTmpl, { NAME: name, VERSION: version, OS: target.os, CPU: target.cpu, BINARY: target.binary })
  );
}

const mainDir = join(outDir, "preflight");

mkdirSync(join(mainDir, "bin"), { recursive: true });
writeFileSync(join(mainDir, "bin", "preflight.js"), wrapper);
writeFileSync(join(mainDir, "package.json"), fill(mainTmpl, { VERSION: version }));

console.log(`generated npm packages for ${version}`);
