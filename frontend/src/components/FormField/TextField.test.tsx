import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { TextField } from "./TextField";

describe("TextField", () => {
    it("is a labelled text box", () => {
        // given
        const ui = <TextField label="Email address" type="email" autoComplete="email" />;

        // when
        renderWithProviders(ui);

        // then
        const input = screen.getByRole("textbox", { name: "Email address" });
        expect(input).toHaveAttribute("type", "email");
        expect(input).toHaveAttribute("autocomplete", "email");
    });

    it("ties its error to the text box", () => {
        // given
        const ui = <TextField label="Password" type="password" error="Use at least 8 characters" />;

        // when
        renderWithProviders(ui);

        // then
        const input = screen.getByLabelText("Password");
        expect(input).toHaveAccessibleDescription("Use at least 8 characters");
        expect(input).toHaveAttribute("aria-invalid", "true");
    });

    it("describes the text box with its hint", () => {
        // given
        const ui = <TextField label="Display name" hint="Other people on your boards see this." />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("textbox", { name: "Display name" })).toHaveAccessibleDescription(
            "Other people on your boards see this.",
        );
    });

    it("reports what is typed through its change handler", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<TextField label="Display name" onChange={onChange} />);

        // when
        await user.type(screen.getByRole("textbox", { name: "Display name" }), "Sam");

        // then
        expect(onChange).toHaveBeenCalledTimes(3);
        expect(screen.getByRole("textbox", { name: "Display name" })).toHaveValue("Sam");
    });

    it("passes native validation through to the input", () => {
        // given
        const ui = <TextField label="Email address" type="email" required maxLength={254} />;

        // when
        renderWithProviders(ui);

        // then
        const input = screen.getByRole("textbox", { name: "Email address" });
        expect(input).toBeRequired();
        expect(input).toHaveAttribute("maxlength", "254");
    });
});
