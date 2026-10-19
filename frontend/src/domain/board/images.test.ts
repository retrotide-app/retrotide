import { describe, expect, it } from "vitest";
import { makeApiError } from "../../test-utils/fixtures";
import { IMAGE_ACCEPT, imageRefusal } from "./images";

describe("IMAGE_ACCEPT", () => {
    it("lets the file picker offer only the formats the server re-encodes", () => {
        // then
        expect(IMAGE_ACCEPT.split(",")).toEqual(["image/png", "image/jpeg", "image/webp"]);
    });
});

describe("imageRefusal", () => {
    it.each([
        { code: "unsupported_image", want: "Only PNG, JPEG and WebP images can be used. GIFs and other files cannot." },
        { code: "image_dimensions", want: "That image is too large. It can have at most 40 megapixels." },
        { code: "images_unavailable", want: "Images cannot be processed right now. Try again in a few minutes." },
    ])("explains $code in plain words", ({ code, want }) => {
        // given
        const error = makeApiError({ status: 400, message: "server words", code });

        // when
        const refusal = imageRefusal(error);

        // then
        expect(refusal).toBe(want);
    });

    it("names the plan's size limit when the upload is over it", () => {
        // given
        const error = Object.assign(
            makeApiError({ status: 402, message: "upgrade required", code: "upgrade_required" }),
            {
                body: {
                    error: "upgrade required",
                    code: "upgrade_required",
                    limit: "card_image_bytes",
                    allowed: 5242880,
                    current: 6000000,
                    plan: "free",
                },
            },
        );

        // when
        const refusal = imageRefusal(error);

        // then
        expect(refusal).toBe("That image is larger than your plan allows. Images can be up to 5 MB.");
    });

    it("says the organisation's image storage is full when that is the limit reached", () => {
        // given
        const error = Object.assign(
            makeApiError({ status: 402, message: "upgrade required", code: "upgrade_required" }),
            {
                body: {
                    error: "upgrade required",
                    code: "upgrade_required",
                    limit: "media_bytes",
                    allowed: 1073741824,
                    current: 1073741824,
                    plan: "free",
                },
            },
        );

        // when
        const refusal = imageRefusal(error);

        // then
        expect(refusal).toBe("Your organisation has used all 1 GB of image storage its plan allows.");
    });

    it("still refuses an oversized upload whose limit the answer left out", () => {
        // given
        const error = makeApiError({ status: 402, message: "upgrade required", code: "upgrade_required" });

        // when
        const refusal = imageRefusal(error);

        // then
        expect(refusal).toBe("That image is larger than your plan allows.");
    });

    it("falls back to the server's own words for any other refusal", () => {
        // given
        const error = makeApiError({
            status: 403,
            message: "only members of the board's organisation can do that",
            code: "members_only",
        });

        // when
        const refusal = imageRefusal(error);

        // then
        expect(refusal).toBe("only members of the board's organisation can do that");
    });
});
