import { describe, expect, it } from "vitest";
import { LEGAL_LINKS, legalPath, offersMarketing } from "./marketing";

describe("legalPath", () => {
    it("puts every legal page under /legal", () => {
        // when
        const paths = LEGAL_LINKS.map(link => legalPath(link.page));

        // then
        expect(paths).toEqual([
            "/legal/privacy",
            "/legal/terms",
            "/legal/acceptable-use",
            "/legal/cookies",
            "/legal/sub-processors",
            "/legal/security",
        ]);
    });
});

describe("offersMarketing", () => {
    it("offers the marketing pages only on the public hosted site", () => {
        // when
        const offered = [
            offersMarketing("hosted", "public"),
            offersMarketing("hosted", "private"),
            offersMarketing("self_hosted", "public"),
            offersMarketing("self_hosted", "private"),
        ];

        // then
        expect(offered).toEqual([true, false, false, false]);
    });
});
