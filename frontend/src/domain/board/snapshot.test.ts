import { describe, expect, it } from "vitest";
import { makeBoardSnapshot } from "../../test-utils/fixtures";
import { keepNewerSnapshot } from "./snapshot";

describe("keepNewerSnapshot", () => {
    it("takes the incoming snapshot when nothing is held", () => {
        // given
        const incoming = makeBoardSnapshot({ version: 2 });

        // when
        const kept = keepNewerSnapshot(undefined, incoming);

        // then
        expect(kept).toBe(incoming);
    });

    it("takes the incoming snapshot when it is newer or the same version", () => {
        // given
        const held = makeBoardSnapshot({ version: 4 });
        const newer = makeBoardSnapshot({ version: 5 });
        const same = makeBoardSnapshot({ version: 4 });

        // when
        const kept = [keepNewerSnapshot(held, newer), keepNewerSnapshot(held, same)];

        // then
        expect(kept[0]).toBe(newer);
        expect(kept[1]).toBe(same);
    });

    it("keeps the held snapshot when a slower answer arrives with an older version", () => {
        // given
        const held = makeBoardSnapshot({ version: 6 });
        const older = makeBoardSnapshot({ version: 5 });

        // when
        const kept = keepNewerSnapshot(held, older);

        // then
        expect(kept).toBe(held);
    });

    it("never compares versions across two boards", () => {
        // given
        const held = makeBoardSnapshot({ board_id: "b1", version: 9 });
        const other = makeBoardSnapshot({ board_id: "b2", version: 1 });

        // when
        const kept = keepNewerSnapshot(held, other);

        // then
        expect(kept).toBe(other);
    });
});
