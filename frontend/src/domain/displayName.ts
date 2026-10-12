export const DISPLAY_NAME_MAX_LENGTH = 40;

export function initialsOf(name: string): string {
    const words = name
        .trim()
        .split(/\s+/)
        .filter(word => word !== "");
    const letters = words.slice(0, 2).map(word => [...word][0] ?? "");

    return letters.join("").toLocaleUpperCase("en-GB");
}

export function accountInitials(name: string, email: string): string {
    const initials = initialsOf(name);

    if (initials !== "") {
        return initials;
    }

    const first = [...email.trim()][0] ?? "";

    return first.toLocaleUpperCase("en-GB");
}
