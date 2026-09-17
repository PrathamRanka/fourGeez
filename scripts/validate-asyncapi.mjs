import { readFile } from "node:fs/promises";
import { Parser } from "@asyncapi/parser";

const source = await readFile("docs/api/asyncapi.yaml", "utf8");
const parser = new Parser();
const { document, diagnostics } = await parser.parse(source);

const errors = diagnostics.filter((diagnostic) => diagnostic.severity === 0);

if (!document || errors.length > 0) {
  for (const diagnostic of diagnostics) {
    const location = diagnostic.range?.start;
    const prefix = location ? `${location.line + 1}:${location.character + 1}` : "unknown";
    console.error(`${prefix} ${diagnostic.message}`);
  }
  process.exit(1);
}

console.log("docs/api/asyncapi.yaml is valid.");
