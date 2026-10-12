import { describe, expect, it } from "vitest";
import * as lazyPages from "./lazyPages";

interface LazyLike {
    _payload: unknown;
    _init: (payload: unknown) => unknown;
}

const LAZY_TYPE = Symbol.for("react.lazy");

async function resolveLazy(component: unknown): Promise<unknown> {
    const lazyComponent = component as LazyLike;

    try {
        return lazyComponent._init(lazyComponent._payload);
    } catch (thrown) {
        await (thrown as Promise<unknown>);

        return lazyComponent._init(lazyComponent._payload);
    }
}

const entries = Object.entries(lazyPages);

describe("lazyPages", () => {
    it("exports nothing but lazy components", () => {
        // given
        const exported = entries;

        // when
        const shapes = exported.map(([, component]) => (component as { $$typeof?: symbol }).$$typeof);

        // then
        expect(shapes.length).toBeGreaterThan(0);
        expect(new Set(shapes)).toEqual(new Set([LAZY_TYPE]));
    });

    it("gives every route a distinct page", () => {
        // given
        const exported = entries;

        // when
        const names = exported.map(([name]) => name);

        // then
        expect(new Set(names).size).toBe(names.length);
    });

    for (const [name, component] of entries) {
        it(`resolves ${name} to a real component`, async () => {
            // given
            const lazyComponent = component;

            // when
            const resolved = await resolveLazy(lazyComponent);

            // then
            expect(resolved).toBeDefined();
            expect(typeof resolved).toBe("function");
        });
    }
});
