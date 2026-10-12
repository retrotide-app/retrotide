interface Positioned {
    id: string;
    position: string;
}

export function compareBytes(a: string, b: string): number {
    const left = Array.from(a, character => character.codePointAt(0) ?? 0);
    const right = Array.from(b, character => character.codePointAt(0) ?? 0);
    const shared = Math.min(left.length, right.length);

    for (let index = 0; index < shared; index++) {
        const difference = left[index] - right[index];

        if (difference !== 0) {
            return difference;
        }
    }

    return left.length - right.length;
}

export function compareByPosition(a: Positioned, b: Positioned): number {
    const byPosition = compareBytes(a.position, b.position);

    if (byPosition !== 0) {
        return byPosition;
    }

    return compareBytes(a.id, b.id);
}
