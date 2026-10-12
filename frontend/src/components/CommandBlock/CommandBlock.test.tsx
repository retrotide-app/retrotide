import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { CommandBlock } from "./CommandBlock";

const { copyText } = vi.hoisted(() => ({ copyText: vi.fn<(text: string) => Promise<void>>() }));

vi.mock("../../platform/clipboard", () => ({ copyText }));

beforeEach(() => {
    copyText.mockReset();
    copyText.mockResolvedValue(undefined);
});

describe("CommandBlock", () => {
    it("shows the command under its label", () => {
        // when
        renderWithProviders(<CommandBlock command="docker compose up -d" label="Start RetroTide" />);

        // then
        expect(screen.getByLabelText("Start RetroTide")).toHaveTextContent("docker compose up -d");
    });

    it("copies the command and says so", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<CommandBlock command="docker compose up -d" label="Start RetroTide" />);

        // when
        await user.click(screen.getByRole("button", { name: "Copy" }));

        // then
        expect(copyText).toHaveBeenCalledExactlyOnceWith("docker compose up -d");
        expect(await screen.findByRole("button", { name: "Copied" })).toBeInTheDocument();
    });

    it("shows the browser's refusal instead of claiming the command was copied", async () => {
        // given
        copyText.mockRejectedValue(new Error("the clipboard is blocked"));
        const user = userEvent.setup();
        renderWithProviders(<CommandBlock command="docker compose up -d" label="Start RetroTide" />);

        // when
        await user.click(screen.getByRole("button", { name: "Copy" }));

        // then
        expect(await screen.findByRole("alert")).toHaveTextContent("the clipboard is blocked");
        expect(screen.getByRole("button", { name: "Copy" })).toBeInTheDocument();
    });
});
