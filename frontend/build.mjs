// Stand-in for the real bundler: copies the placeholder page into dist so the
// Go embed has something to serve. Replaced when the GUI screens land.
import { cp, mkdir, readdir, rm } from "node:fs/promises";

await mkdir("dist", { recursive: true });
// .gitkeep is committed so a fresh clone can still compile the embed.
for (const name of await readdir("dist")) {
  if (name !== ".gitkeep") await rm(`dist/${name}`, { recursive: true, force: true });
}
await cp("index.html", "dist/index.html");
