import { describe, expect, it } from "vitest";
import { UUID_BYTE_LENGTH, uuidV4FromBytes } from "./uuid";

const V4_SHAPE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("uuidV4FromBytes", () => {
    it("sets the version and variant bits and keeps every other bit", () => {
        // given
        const random = new Uint8Array(UUID_BYTE_LENGTH).fill(0xff);

        // when
        const id = uuidV4FromBytes(random);

        // then
        expect(id).toBe("ffffffff-ffff-4fff-bfff-ffffffffffff");
    });

    it("writes zero bytes as a v4 UUID too", () => {
        // given
        const random = new Uint8Array(UUID_BYTE_LENGTH);

        // when
        const id = uuidV4FromBytes(random);

        // then
        expect(id).toBe("00000000-0000-4000-8000-000000000000");
        expect(id).toMatch(V4_SHAPE);
    });

    it("lays the bytes out in order across the five groups", () => {
        // given
        const random = Uint8Array.from({ length: UUID_BYTE_LENGTH }, (_, index) => index);

        // when
        const id = uuidV4FromBytes(random);

        // then
        expect(id).toBe("00010203-0405-4607-8809-0a0b0c0d0e0f");
    });

    it("leaves the caller's bytes as they were", () => {
        // given
        const random = new Uint8Array(UUID_BYTE_LENGTH).fill(0xff);

        // when
        uuidV4FromBytes(random);

        // then
        expect(Array.from(random)).toEqual(new Array<number>(UUID_BYTE_LENGTH).fill(0xff));
    });

    it("refuses any other number of bytes", () => {
        // given
        const short = new Uint8Array(8);

        // when
        const make = () => uuidV4FromBytes(short);

        // then
        expect(make).toThrow(RangeError);
    });
});
