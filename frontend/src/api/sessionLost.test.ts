import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    clearSessionLost,
    isSessionLost,
    isUserSessionHeld,
    markSessionLost,
    setUserSessionHeld,
    subscribeSessionLost,
} from "./sessionLost";

beforeEach(() => {
    clearSessionLost();
    setUserSessionHeld(false);
});

describe("user session held", () => {
    it("starts with no user session held", () => {
        // given
        const fresh = isUserSessionHeld;

        // when
        const held = fresh();

        // then
        expect(held).toBe(false);
    });

    it("remembers whether a user session is held", () => {
        // given
        setUserSessionHeld(true);

        // when
        const held = isUserSessionHeld();

        // then
        expect(held).toBe(true);
    });

    it("never tells the session lost listeners when the held flag changes", () => {
        // given
        const listener = vi.fn();
        subscribeSessionLost(listener);

        // when
        setUserSessionHeld(true);
        setUserSessionHeld(false);

        // then
        expect(listener).not.toHaveBeenCalled();
        expect(isSessionLost()).toBe(false);
    });
});

describe("sessionLost", () => {
    it("starts with the session intact", () => {
        // given
        const fresh = isSessionLost;

        // when
        const lost = fresh();

        // then
        expect(lost).toBe(false);
    });

    it("tells its listeners once when the session is lost", () => {
        // given
        const listener = vi.fn();
        subscribeSessionLost(listener);

        // when
        markSessionLost();
        markSessionLost();

        // then
        expect(isSessionLost()).toBe(true);
        expect(listener).toHaveBeenCalledOnce();
    });

    it("tells its listeners when the session is restored", () => {
        // given
        markSessionLost();
        const listener = vi.fn();
        subscribeSessionLost(listener);

        // when
        clearSessionLost();
        clearSessionLost();

        // then
        expect(isSessionLost()).toBe(false);
        expect(listener).toHaveBeenCalledOnce();
    });

    it("stops telling a listener that has unsubscribed", () => {
        // given
        const listener = vi.fn();
        const unsubscribe = subscribeSessionLost(listener);

        // when
        unsubscribe();
        markSessionLost();

        // then
        expect(listener).not.toHaveBeenCalled();
    });
});
