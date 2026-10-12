import { describe, expect, it, vi } from "vitest";
import { queryKeys } from "../api/queryKeys";
import { expectInvalidated } from "./query";
import { createTestQueryClient } from "./render";

function spiedClient() {
    const queryClient = createTestQueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    return { queryClient, invalidate };
}

describe("expectInvalidated", () => {
    it("passes when the key was invalidated", async () => {
        // given
        const { queryClient } = spiedClient();

        // when
        await queryClient.invalidateQueries({ queryKey: queryKeys.siteInfo() });

        // then
        expectInvalidated(queryClient, queryKeys.siteInfo());
    });

    it("passes when the key was one invalidation among several", async () => {
        // given
        const { queryClient } = spiedClient();

        // when
        await queryClient.invalidateQueries({ queryKey: queryKeys.siteInfo() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.auth.session() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.auth.all });

        // then
        expectInvalidated(queryClient, queryKeys.auth.session());
    });

    it("fails when a different key was invalidated", async () => {
        // given
        const { queryClient } = spiedClient();

        // when
        await queryClient.invalidateQueries({ queryKey: queryKeys.siteInfo() });

        // then
        expect(() => expectInvalidated(queryClient, queryKeys.auth.session())).toThrow();
    });

    it("fails when nothing was invalidated", () => {
        // given
        const { queryClient } = spiedClient();

        // when
        // then
        expect(() => expectInvalidated(queryClient, queryKeys.auth.session())).toThrow();
    });

    it("matches on the whole key, not a prefix of it", async () => {
        // given
        const { queryClient } = spiedClient();

        // when
        await queryClient.invalidateQueries({ queryKey: queryKeys.auth.all });

        // then
        expect(() => expectInvalidated(queryClient, queryKeys.auth.session())).toThrow();
    });

    it("ignores an invalidation that carried extra options", async () => {
        // given
        const { queryClient } = spiedClient();

        // when
        await queryClient.invalidateQueries({ queryKey: queryKeys.auth.session(), exact: true });

        // then
        expect(() => expectInvalidated(queryClient, queryKeys.auth.session())).toThrow();
    });

    it("says so when the client was never spied on", async () => {
        // given
        const queryClient = createTestQueryClient();

        // when
        await queryClient.invalidateQueries({ queryKey: queryKeys.auth.session() });

        // then
        expect(() => expectInvalidated(queryClient, queryKeys.auth.session())).toThrow(/vi\.spyOn/);
    });

    it("reads the spy at assertion time, so the act may happen after the helper is imported", async () => {
        // given
        const { queryClient, invalidate } = spiedClient();

        // when
        await queryClient.invalidateQueries({ queryKey: queryKeys.auth.session() });

        // then
        expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.auth.session() });
        expectInvalidated(queryClient, queryKeys.auth.session());
    });
});
