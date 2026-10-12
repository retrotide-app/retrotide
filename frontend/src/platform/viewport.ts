export const NARROW_VIEWPORT_QUERY = "(max-width: 720px)";

export function isNarrowViewport(): boolean {
    return window.matchMedia(NARROW_VIEWPORT_QUERY).matches;
}

export function subscribeNarrowViewport(onChange: () => void): () => void {
    const media = window.matchMedia(NARROW_VIEWPORT_QUERY);

    media.addEventListener("change", onChange);

    return () => {
        media.removeEventListener("change", onChange);
    };
}
