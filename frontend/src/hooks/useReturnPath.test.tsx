import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";
import { useReturnPath } from "./useReturnPath";

function arrivedWith(state: unknown) {
    return ({ children }: { children: ReactNode }) => (
        <MemoryRouter initialEntries={[{ pathname: "/login", state }]}>{children}</MemoryRouter>
    );
}

describe("useReturnPath", () => {
    it("sends the user back to the page a guard turned them away from", () => {
        // given
        const wrapper = arrivedWith({ from: "/admin/users?search=sam" });

        // when
        const { result } = renderHook(useReturnPath, { wrapper });

        // then
        expect(result.current).toBe("/admin/users?search=sam");
    });

    it("sends the user home when nothing turned them away", () => {
        // given
        const wrapper = arrivedWith(null);

        // when
        const { result } = renderHook(useReturnPath, { wrapper });

        // then
        expect(result.current).toBe("/");
    });

    it("sends the user home rather than off the site", () => {
        // given
        const wrapper = arrivedWith({ from: "//evil.example" });

        // when
        const { result } = renderHook(useReturnPath, { wrapper });

        // then
        expect(result.current).toBe("/");
    });
});
