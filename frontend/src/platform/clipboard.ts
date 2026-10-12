export async function copyText(text: string): Promise<void> {
    if (typeof navigator === "undefined" || navigator.clipboard === undefined) {
        throw new Error("This browser does not let the page copy text. Select the text and copy it yourself.");
    }

    await navigator.clipboard.writeText(text);
}
