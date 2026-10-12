import { toPng } from "html-to-image";

export async function exportNodeAsPng(node: HTMLElement, fileName: string): Promise<void> {
    const dataUrl = await toPng(node);
    const anchor = document.createElement("a");

    anchor.href = dataUrl;
    anchor.download = fileName;
    document.body.append(anchor);
    anchor.click();
    anchor.remove();
}
