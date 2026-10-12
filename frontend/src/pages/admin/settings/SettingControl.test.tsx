import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SECRET_MASK, type SiteSettingField } from "../../../domain/siteSettings";
import { renderWithProviders } from "../../../test-utils/render";
import { SettingControl } from "./SettingControl";

interface ControlOptions {
    field: SiteSettingField;
    value?: string;
    savedValue?: string;
    replacing?: boolean;
    disabled?: boolean;
}

function renderControl({ field, value = "", savedValue = "", replacing = false, disabled = false }: ControlOptions) {
    const handlers = { onChange: vi.fn(), onReplace: vi.fn(), onKeep: vi.fn() };

    renderWithProviders(
        <SettingControl
            field={field}
            value={value}
            savedValue={savedValue}
            replacing={replacing}
            disabled={disabled}
            {...handlers}
        />,
    );

    return handlers;
}

const siteName: SiteSettingField = { key: "site_name", label: "Site name", kind: "text", hint: "Shown in the header." };
const port: SiteSettingField = { key: "smtp_port", label: "SMTP port", kind: "number" };
const maintenance: SiteSettingField = {
    key: "maintenance_mode",
    label: "Maintenance mode",
    kind: "toggle",
    hint: "Only staff can use the site.",
};
const logLevel: SiteSettingField = {
    key: "log_level",
    label: "Log level",
    kind: "choice",
    options: [
        { value: "info", label: "Info" },
        { value: "debug", label: "Debug" },
    ],
};
const secret: SiteSettingField = { key: "smtp_password", label: "SMTP password", kind: "secret" };

describe("SettingControl", () => {
    it("edits a text setting in a labelled field with its hint", async () => {
        // given
        const user = userEvent.setup();
        const handlers = renderControl({ field: siteName, value: "RetroTide" });

        // when
        await user.type(screen.getByRole("textbox", { name: "Site name" }), "!");

        // then
        expect(screen.getByRole("textbox", { name: "Site name" })).toHaveAccessibleDescription("Shown in the header.");
        expect(handlers.onChange).toHaveBeenLastCalledWith("RetroTide!");
    });

    it("edits a number setting in a number field", () => {
        // given
        const field = port;

        // when
        renderControl({ field, value: "25" });

        // then
        expect(screen.getByRole("spinbutton", { name: "SMTP port" })).toHaveValue(25);
    });

    it("switches a toggle setting that is off to true", async () => {
        // given
        const user = userEvent.setup();
        const handlers = renderControl({ field: maintenance, value: "false" });

        // when
        await user.click(screen.getByRole("switch", { name: "Maintenance mode" }));

        // then
        expect(handlers.onChange).toHaveBeenCalledWith("true");
    });

    it("shows a toggle setting that is on as on, and switches it to false", async () => {
        // given
        const user = userEvent.setup();
        const handlers = renderControl({ field: maintenance, value: "true" });
        const toggle = screen.getByRole("switch", { name: "Maintenance mode" });
        const onBefore = toggle.getAttribute("aria-checked");

        // when
        await user.click(toggle);

        // then
        expect(onBefore).toBe("true");
        expect(handlers.onChange).toHaveBeenCalledWith("false");
    });

    it("picks a choice setting from its options", async () => {
        // given
        const user = userEvent.setup();
        const handlers = renderControl({ field: logLevel, value: "info" });

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Log level" }), "debug");

        // then
        expect(handlers.onChange).toHaveBeenCalledWith("debug");
    });

    it("keeps a saved choice the options do not list", () => {
        // given
        const field = logLevel;

        // when
        renderControl({ field, value: "warn" });

        // then
        const select = screen.getByRole("combobox", { name: "Log level" });
        expect(select).toHaveValue("warn");
        expect(within(select).getAllByRole("option")).toHaveLength(3);
    });

    it("hides a saved secret and offers to replace it", async () => {
        // given
        const user = userEvent.setup();
        const handlers = renderControl({ field: secret, value: SECRET_MASK, savedValue: SECRET_MASK });

        // when
        await user.click(screen.getByRole("button", { name: "Replace SMTP password" }));

        // then
        const saved = screen.getByLabelText("SMTP password");
        expect(saved).toHaveAttribute("readonly");
        expect(saved).toHaveValue("••••••••••••");
        expect(screen.getByText("Saved. It is never shown again.")).toBeInTheDocument();
        expect(screen.queryByDisplayValue(SECRET_MASK)).not.toBeInTheDocument();
        expect(handlers.onReplace).toHaveBeenCalledOnce();
    });

    it("takes a replacement secret in a password field, with a way back", async () => {
        // given
        const user = userEvent.setup();
        const handlers = renderControl({ field: secret, value: "", savedValue: SECRET_MASK, replacing: true });
        const input = screen.getByLabelText("SMTP password");

        // when
        await user.type(input, "s");
        await user.click(screen.getByRole("button", { name: "Keep saved value for SMTP password" }));

        // then
        expect(input).toHaveAttribute("type", "password");
        expect(input).toHaveAccessibleDescription("Leave it empty to remove the saved value.");
        expect(handlers.onChange).toHaveBeenCalledWith("s");
        expect(handlers.onKeep).toHaveBeenCalledOnce();
    });

    it("takes a secret that was never set in a password field", () => {
        // given
        const field = secret;

        // when
        renderControl({ field, value: "", savedValue: "" });

        // then
        expect(screen.getByLabelText("SMTP password")).toHaveAttribute("type", "password");
        expect(screen.queryByRole("button", { name: "Replace SMTP password" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: /Keep saved value/ })).not.toBeInTheDocument();
    });

    it("holds every kind of control while the settings save", () => {
        // given
        const disabled = true;

        // when
        renderControl({ field: siteName, disabled });
        renderControl({ field: maintenance, disabled });
        renderControl({ field: logLevel, value: "info", disabled });
        renderControl({ field: secret, savedValue: SECRET_MASK, disabled });

        // then
        expect(screen.getByRole("textbox", { name: "Site name" })).toBeDisabled();
        expect(screen.getByRole("switch", { name: "Maintenance mode" })).toBeDisabled();
        expect(screen.getByRole("combobox", { name: "Log level" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Replace SMTP password" })).toBeDisabled();
    });
});
