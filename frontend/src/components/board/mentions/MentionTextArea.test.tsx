import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardMembership } from "../../../hooks/board/useBoardMembership";
import { BOARD_ID, makeApiError, makePerson } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { PersonView } from "../../../types/api";
import { MentionTextArea } from "./MentionTextArea";

const { useBoardMembership } = vi.hoisted(() => ({ useBoardMembership: vi.fn() }));

vi.mock("../../../hooks/board/useBoardMembership", () => ({ useBoardMembership }));

const sam = makePerson({ user_id: "u1", display_name: "Sam" });
const samLee = makePerson({ user_id: "u2", display_name: "Sam Lee" });

function membership(overrides: Partial<BoardMembership> = {}): BoardMembership {
    return { people: [sam, samLee], member: true, failure: null, retry: vi.fn(), ...overrides };
}

interface HarnessProps {
    onKeyDown: () => void;
    onPicked: (mentions: PersonView[]) => void;
}

function Harness({ onKeyDown, onPicked }: HarnessProps) {
    const [text, setText] = useState("");
    const [mentions, setMentions] = useState<readonly PersonView[]>([]);

    return (
        <MentionTextArea
            boardId={BOARD_ID}
            aria-label="Card text"
            placeholder="Write a card"
            text={text}
            mentions={mentions}
            onTextChange={(nextText, nextMentions) => {
                setText(nextText);
                setMentions(nextMentions);
                onPicked(nextMentions);
            }}
            onKeyDown={onKeyDown}
        />
    );
}

function renderArea() {
    const onKeyDown = vi.fn();
    const onPicked = vi.fn();

    renderWithProviders(<Harness onKeyDown={onKeyDown} onPicked={onPicked} />);

    return { onKeyDown, onPicked };
}

beforeEach(() => {
    vi.clearAllMocks();
    useBoardMembership.mockReturnValue(membership());
});

describe("MentionTextArea", () => {
    it("asks for the board's people and says @ mentions someone", () => {
        // when
        renderArea();

        // then
        expect(useBoardMembership).toHaveBeenCalledWith(BOARD_ID, true);
        expect(screen.getByRole("combobox", { name: "Card text" })).toHaveAttribute(
            "placeholder",
            "Write a card. Type @ to mention someone",
        );
    });

    it("moves through the matching people with the arrow keys and picks the highlighted one", async () => {
        // given
        const user = userEvent.setup();
        const { onPicked } = renderArea();

        // when
        await user.type(screen.getByRole("combobox", { name: "Card text" }), "Ask @sam");
        await user.keyboard("{ArrowDown}{Enter}");

        // then
        expect(screen.getByRole("combobox", { name: "Card text" })).toHaveValue("Ask @Sam Lee ");
        expect(onPicked).toHaveBeenLastCalledWith([samLee]);
    });

    it("keeps the keys the list uses from the box's own handler while the list is open", async () => {
        // given
        const user = userEvent.setup();
        const { onKeyDown } = renderArea();

        // when
        await user.type(screen.getByRole("combobox", { name: "Card text" }), "@");
        onKeyDown.mockClear();
        await user.keyboard("{Escape}");
        const whileOpen = onKeyDown.mock.calls.length;
        await user.keyboard("{Escape}");

        // then
        expect(whileOpen).toBe(0);
        expect(onKeyDown).toHaveBeenCalledOnce();
    });

    it("leaves a plain text box when the viewer may name nobody", () => {
        // given
        useBoardMembership.mockReturnValue(membership({ people: null, member: false }));

        // when
        renderArea();

        // then
        expect(screen.getByRole("textbox", { name: "Card text" })).toHaveAttribute("placeholder", "Write a card");
    });

    it("says why the people could not be read, and tries again", async () => {
        // given
        const retry = vi.fn();
        useBoardMembership.mockReturnValue(
            membership({
                people: null,
                member: false,
                failure: makeApiError({ status: 503, message: "service unavailable" }),
                retry,
            }),
        );
        const user = userEvent.setup();
        renderArea();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
        expect(retry).toHaveBeenCalledOnce();
    });
});
