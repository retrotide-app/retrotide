import DOMPurify from "dompurify";
import { marked } from "marked";

export function renderMarkdown(source: string): string {
    const html = marked(source, { async: false, gfm: true });

    return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
}
