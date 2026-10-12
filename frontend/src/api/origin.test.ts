import { describe, expect, it } from "vitest";
import { API_PREFIX, apiUrl, socketUrl } from "./origin";

describe("API_PREFIX", () => {
    it("is the versioned root every API route hangs off", () => {
        // then
        expect(API_PREFIX).toBe("/api/v1");
    });
});

describe("apiUrl", () => {
    it("puts the path under the versioned prefix, relative to the page's own origin", () => {
        // given
        const path = "/site-info";

        // when
        const url = apiUrl(path);

        // then
        expect(url).toBe("/api/v1/site-info");
    });

    it("keeps a query string the caller built", () => {
        // given
        const path = "/boards?limit=20";

        // when
        const url = apiUrl(path);

        // then
        expect(url).toBe("/api/v1/boards?limit=20");
    });
});

describe("socketUrl", () => {
    it("opens a plain ws socket when the page is served over http", () => {
        // given
        const origin = "http://localhost:5174";

        // when
        const url = socketUrl(origin);

        // then
        expect(url).toBe("ws://localhost:5174/api/v1/ws");
    });

    it("opens a secure wss socket when the page is served over https", () => {
        // given
        const origin = "https://retrotide.app";

        // when
        const url = socketUrl(origin);

        // then
        expect(url).toBe("wss://retrotide.app/api/v1/ws");
    });

    it("never carries a query string, because the socket authenticates with the same cookies", () => {
        // given
        const origins = ["http://localhost:5174", "https://retrotide.app"];

        // when
        const urls = origins.map(socketUrl);

        // then
        for (const url of urls) {
            expect(url).not.toContain("?");
            expect(new URL(url).search).toBe("");
        }
    });
});
