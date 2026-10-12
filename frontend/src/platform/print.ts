export function printPage(): void {
    if (typeof window.print !== "function") {
        throw new Error("This browser cannot print the page. Use its own menu to print it or save it as a PDF.");
    }

    window.print();
}
