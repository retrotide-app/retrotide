export const UUID_BYTE_LENGTH = 16;

const GROUP_ENDS = [4, 6, 8, 10, 16];

export function uuidV4FromBytes(random: Uint8Array): string {
    if (random.length !== UUID_BYTE_LENGTH) {
        throw new RangeError(`a UUID needs ${UUID_BYTE_LENGTH} random bytes, not ${random.length}`);
    }

    const bytes = Uint8Array.from(random);
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;

    const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, "0"));
    const groups: string[] = [];
    let start = 0;

    for (const end of GROUP_ENDS) {
        groups.push(hex.slice(start, end).join(""));
        start = end;
    }

    return groups.join("-");
}
