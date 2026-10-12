import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { ToggleSwitch } from "./ToggleSwitch";

describe("ToggleSwitch", () => {
    it("exposes itself as a switch named after its label", () => {
        // given
        const label = "Show vote counts";

        // when
        renderWithProviders(<ToggleSwitch enabled={false} onChange={vi.fn()} label={label} />);

        // then
        expect(screen.getByRole("switch", { name: label })).toBeInTheDocument();
    });

    it("takes its accessible name from ariaLabel when one is given", () => {
        // given
        const ariaLabel = "Show vote counts to everyone on the board";

        // when
        renderWithProviders(
            <ToggleSwitch enabled={false} onChange={vi.fn()} label="Show vote counts" ariaLabel={ariaLabel} />,
        );

        // then
        expect(screen.getByRole("switch", { name: ariaLabel })).toBeInTheDocument();
    });

    it("reports that it is off when it is not enabled", () => {
        // given
        const enabled = false;

        // when
        renderWithProviders(<ToggleSwitch enabled={enabled} onChange={vi.fn()} label="Show vote counts" />);

        // then
        expect(screen.getByRole("switch", { name: "Show vote counts" })).toHaveAttribute("aria-checked", "false");
    });

    it("reports that it is on when it is enabled", () => {
        // given
        const enabled = true;

        // when
        renderWithProviders(<ToggleSwitch enabled={enabled} onChange={vi.fn()} label="Show vote counts" />);

        // then
        expect(screen.getByRole("switch", { name: "Show vote counts" })).toHaveAttribute("aria-checked", "true");
    });

    it("asks to be switched on when it is clicked while off", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ToggleSwitch enabled={false} onChange={onChange} label="Show vote counts" />);

        // when
        await user.click(screen.getByRole("switch", { name: "Show vote counts" }));

        // then
        expect(onChange).toHaveBeenCalledWith(true);
    });

    it("asks to be switched off when it is clicked while on", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ToggleSwitch enabled onChange={onChange} label="Show vote counts" />);

        // when
        await user.click(screen.getByRole("switch", { name: "Show vote counts" }));

        // then
        expect(onChange).toHaveBeenCalledWith(false);
    });

    it("can be operated from the keyboard", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ToggleSwitch enabled={false} onChange={onChange} label="Show vote counts" />);

        // when
        await user.tab();
        await user.keyboard("{Enter}");
        await user.keyboard(" ");

        // then
        expect(screen.getByRole("switch", { name: "Show vote counts" })).toHaveFocus();
        expect(onChange).toHaveBeenCalledTimes(2);
        expect(onChange).toHaveBeenNthCalledWith(2, true);
    });

    it("shows the description beneath the label when one is given", () => {
        // given
        const description = "Everyone on the board sees how many votes each card has";

        // when
        renderWithProviders(
            <ToggleSwitch enabled={false} onChange={vi.fn()} label="Show vote counts" description={description} />,
        );

        // then
        expect(screen.getByText("Show vote counts")).toBeInTheDocument();
        expect(screen.getByText(description)).toBeInTheDocument();
    });

    it("leaves out the description when none is given", () => {
        // given
        const description = undefined;

        // when
        renderWithProviders(
            <ToggleSwitch enabled={false} onChange={vi.fn()} label="Show vote counts" description={description} />,
        );

        // then
        expect(screen.getByRole("switch", { name: "Show vote counts" }).textContent).toBe("Show vote counts");
    });

    it("stays usable when nothing disables it", () => {
        // given
        const disabled = undefined;

        // when
        renderWithProviders(
            <ToggleSwitch enabled={false} onChange={vi.fn()} label="Show vote counts" disabled={disabled} />,
        );

        // then
        expect(screen.getByRole("switch", { name: "Show vote counts" })).toBeEnabled();
    });

    it("refuses to be operated once it is disabled", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ToggleSwitch enabled={false} onChange={onChange} label="Show vote counts" disabled />);

        // when
        await user.click(screen.getByRole("switch", { name: "Show vote counts" }));

        // then
        expect(screen.getByRole("switch", { name: "Show vote counts" })).toBeDisabled();
        expect(onChange).not.toHaveBeenCalled();
    });

    it("still reports the state it is showing while disabled", () => {
        // given
        const enabled = true;

        // when
        renderWithProviders(<ToggleSwitch enabled={enabled} onChange={vi.fn()} label="Show vote counts" disabled />);

        // then
        expect(screen.getByRole("switch", { name: "Show vote counts" })).toHaveAttribute("aria-checked", "true");
    });

    it("does not submit a surrounding form when it is toggled", async () => {
        // given
        const onSubmit = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <form onSubmit={onSubmit}>
                <ToggleSwitch enabled={false} onChange={vi.fn()} label="Show vote counts" />
            </form>,
        );

        // when
        await user.click(screen.getByRole("switch", { name: "Show vote counts" }));

        // then
        expect(onSubmit).not.toHaveBeenCalled();
    });
});
