import { act, renderHook } from "@testing-library/react";
import type { KeyboardEvent } from "react";
import { describe, expect, it, vi } from "vitest";
import { makePerson } from "../../test-utils/fixtures";
import type { PersonView } from "../../types/api";
import { useMentionPicker } from "./useMentionPicker";

const sam = makePerson({ user_id: "u1", display_name: "Sam" });
const ali = makePerson({ user_id: "u3", display_name: "Ali Khan" });

function keyDown(key: string): KeyboardEvent<HTMLTextAreaElement> {
    return { key, preventDefault: vi.fn(), stopPropagation: vi.fn() } as unknown as KeyboardEvent<HTMLTextAreaElement>;
}

function renderPicker(text: string, people: readonly PersonView[] | null) {
    const onChange = vi.fn();
    const onKeyDown = vi.fn();
    const rendered = renderHook(() =>
        useMentionPicker<HTMLTextAreaElement>({ text, mentions: [], people, onChange, onKeyDown }),
    );

    return { ...rendered, onChange, onKeyDown };
}

describe("useMentionPicker", () => {
    it("opens on the name being typed after an @ and lists the members it matches", () => {
        // when
        const { result } = renderPicker("Ask @al", [sam, ali]);

        // then
        expect(result.current.open).toBe(true);
        expect(result.current.suggestions).toEqual([ali]);
        expect(result.current.combobox["aria-expanded"]).toBe(true);
        expect(result.current.combobox["aria-activedescendant"]).toBe(result.current.optionIds[0]);
    });

    it("writes the picked member's name into the text and names them", () => {
        // given
        const { result, onChange } = renderPicker("Ask @al", [sam, ali]);

        // when
        act(() => {
            result.current.pick(ali);
        });

        // then
        expect(onChange).toHaveBeenCalledExactlyOnceWith("Ask @Ali Khan ", [ali]);
    });

    it("picks on Enter instead of handing the key on", () => {
        // given
        const { result, onChange, onKeyDown } = renderPicker("Ask @al", [sam, ali]);
        const enter = keyDown("Enter");

        // when
        act(() => {
            result.current.combobox.onKeyDown(enter);
        });

        // then
        expect(enter.preventDefault).toHaveBeenCalledOnce();
        expect(onChange).toHaveBeenCalledExactlyOnceWith("Ask @Ali Khan ", [ali]);
        expect(onKeyDown).not.toHaveBeenCalled();
    });

    it("stays closed and hands every key on when nobody can be named", () => {
        // given
        const { result, onKeyDown } = renderPicker("Ask @al", null);
        const enter = keyDown("Enter");

        // when
        act(() => {
            result.current.combobox.onKeyDown(enter);
        });

        // then
        expect(result.current.open).toBe(false);
        expect(onKeyDown).toHaveBeenCalledExactlyOnceWith(enter);
    });
});
