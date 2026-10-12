import { readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const sourceDir = resolve(root, "src");
const tokenFile = resolve(sourceDir, "styles/variables.css");

const colourLiteral =
    /#(?:[0-9a-f]{8}|[0-9a-f]{6}|[0-9a-f]{3,4})(?![0-9a-z_-])|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(/gi;

const stylesheets = readdirSync(sourceDir, { recursive: true, withFileTypes: true })
    .filter(entry => entry.isFile() && entry.name.endsWith(".css"))
    .map(entry => join(entry.parentPath, entry.name))
    .filter(file => file !== tokenFile)
    .sort();

const offences = [];

for (const file of stylesheets) {
    const lines = readFileSync(file, "utf8").split(/\r?\n/);
    const shown = relative(root, file).replaceAll("\\", "/");

    for (const [index, line] of lines.entries()) {
        for (const match of line.matchAll(colourLiteral)) {
            offences.push(`${shown}:${index + 1}:${match.index + 1}: ${match[0]}`);
        }
    }
}

if (offences.length > 0) {
    console.error("colour literals outside src/styles/variables.css; use a token from it instead:");
    console.error(offences.join("\n"));
    process.exitCode = 1;
} else {
    console.log(`${stylesheets.length} stylesheets checked, no colour literal outside src/styles/variables.css`);
}
