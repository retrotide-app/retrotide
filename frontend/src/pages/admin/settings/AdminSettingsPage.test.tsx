import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SECRET_MASK } from "../../../domain/siteSettings";
import { makeApiError, makeMutationState, makeQueryState, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminSettingsResponse } from "../../../types/api";
import { AdminSettingsPage } from "./AdminSettingsPage";

const { useAdminSettings, useUpdateAdminSettings, useSendTestEmail } = vi.hoisted(() => ({
    useAdminSettings: vi.fn(),
    useUpdateAdminSettings: vi.fn(),
    useSendTestEmail: vi.fn(),
}));

vi.mock("../../../hooks/queries/admin", () => ({ useAdminSettings }));
vi.mock("../../../hooks/mutations/admin", () => ({ useUpdateAdminSettings, useSendTestEmail }));

const savedSettings: Record<string, string> = {
    base_url: "https://retrotide.test",
    site_name: "RetroTide",
    log_level: "info",
    otlp_endpoint: "",
    pyroscope_url: "",
    max_body_size: "1048576",
    maintenance_mode: "false",
    metrics_token: "",
    valkey_url: "",
    cache_in_memory_max_mb: "128",
    site_mode: "public",
    min_password_length: "8",
    session_duration_days: "30",
    turnstile_enabled: "false",
    turnstile_site_key: "",
    turnstile_secret_key: "",
    dronebl_enabled: "false",
    dronebl_allowlist: "",
    dronebl_ignored_classes: "",
    email_provider: "smtp",
    smtp_host: "mail.example.test",
    smtp_port: "25",
    smtp_from: "noreply@example.test",
    smtp_username: "",
    smtp_password: SECRET_MASK,
    cloudflare_account_id: "",
    cloudflare_api_token: "",
    cloudflare_email_from: "",
    google_client_id: "",
    google_client_secret: "",
};

const SAVE_GENERAL = "Save general settings";
const SAVE_EMAIL = "Save email settings";

function showSettings(settings: Record<string, string> = savedSettings) {
    useAdminSettings.mockReturnValue(makeQueryState<AdminSettingsResponse>({ data: { settings } }));
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

function renderPage(section?: string) {
    return renderWithProviders(<AdminSettingsPage />, {
        user: makeSessionUser({ email: "sam@example.test" }),
        permissions: ["view_admin_panel", "manage_settings"],
        route: section === undefined ? "/admin/settings" : `/admin/settings?section=${section}`,
    });
}

beforeEach(() => {
    showSettings();
    useUpdateAdminSettings.mockReturnValue(makeMutationState());
    useSendTestEmail.mockReturnValue(makeMutationState());
});

describe("AdminSettingsPage", () => {
    it("lists every section and opens on General, showing only its settings", () => {
        // given
        const sections = [
            "General",
            "Accounts",
            "Google sign-in",
            "Turnstile",
            "DroneBL",
            "Email",
            "Limits and cache",
            "Logging and monitoring",
        ];

        // when
        renderPage();

        // then
        const nav = screen.getByRole("navigation", { name: "Settings sections" });
        expect(
            within(nav)
                .getAllByRole("link")
                .map(link => link.textContent),
        ).toEqual(sections);
        expect(within(nav).getByRole("link", { name: "General" })).toHaveAttribute("aria-current", "page");
        expect(screen.getByRole("region", { name: "General" })).toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "Email" })).not.toBeInTheDocument();
        expect(screen.getByRole("textbox", { name: "Site name" })).toHaveValue("RetroTide");
    });

    it("moves to another section from the list", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("link", { name: "Email" }));

        // then
        expect(screen.getByRole("region", { name: "Email" })).toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "General" })).not.toBeInTheDocument();
    });

    it("opens the section named in the address, and General for one it does not know", () => {
        // when
        renderPage("nonsense");

        // then
        expect(screen.getByRole("region", { name: "General" })).toBeInTheDocument();
    });

    it("says it is loading while the settings are on their way", () => {
        // given
        useAdminSettings.mockReturnValue(makeQueryState<AdminSettingsResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading settings...")).toBeInTheDocument();
    });

    it("shows the server's reason when the settings could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<AdminSettingsResponse>({
            error: makeApiError({ status: 500, message: "failed to load the settings" }),
        });
        useAdminSettings.mockReturnValue(failed);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to load the settings");
        expect(screen.queryByRole("button", { name: SAVE_GENERAL })).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("holds the section's save button until something changes", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("button", { name: SAVE_GENERAL })).toBeDisabled();
    });

    it("saves only the settings that changed", async () => {
        // given
        const update = makeMutationState();
        useUpdateAdminSettings.mockReturnValue(update);
        const user = userEvent.setup();
        renderPage();

        // when
        const siteName = screen.getByRole("textbox", { name: "Site name" });
        await user.clear(siteName);
        await user.type(siteName, "Retro");
        await user.click(screen.getByRole("switch", { name: "Maintenance mode" }));
        await user.selectOptions(screen.getByRole("combobox", { name: "Site mode" }), "private");
        await user.click(screen.getByRole("button", { name: SAVE_GENERAL }));

        // then
        expect(update.mutate).toHaveBeenCalledWith(
            { settings: { site_name: "Retro", maintenance_mode: "true", site_mode: "private" } },
            expect.anything(),
        );
    });

    it("counts an edit put back to the saved value as no change", async () => {
        // given
        const user = userEvent.setup();
        renderPage();
        const siteName = screen.getByRole("textbox", { name: "Site name" });

        // when
        await user.type(siteName, "!");
        await user.type(siteName, "{Backspace}");

        // then
        expect(screen.getByRole("button", { name: SAVE_GENERAL })).toBeDisabled();
    });

    it("never sends a masked secret back, and sends its replacement", async () => {
        // given
        const update = makeMutationState();
        useUpdateAdminSettings.mockReturnValue(update);
        const user = userEvent.setup();
        renderPage("email");

        // when
        await user.click(screen.getByRole("button", { name: "Replace SMTP password" }));
        await user.type(screen.getByLabelText("SMTP password"), "new-secret");
        await user.click(screen.getByRole("button", { name: SAVE_EMAIL }));

        // then
        expect(update.mutate).toHaveBeenCalledWith({ settings: { smtp_password: "new-secret" } }, expect.anything());
    });

    it("removes a saved secret when its replacement is left empty", async () => {
        // given
        const update = makeMutationState();
        useUpdateAdminSettings.mockReturnValue(update);
        const user = userEvent.setup();
        renderPage("email");

        // when
        await user.click(screen.getByRole("button", { name: "Replace SMTP password" }));
        await user.click(screen.getByRole("button", { name: SAVE_EMAIL }));

        // then
        expect(update.mutate).toHaveBeenCalledWith({ settings: { smtp_password: "" } }, expect.anything());
    });

    it("keeps a saved secret when its replacement is abandoned", async () => {
        // given
        const user = userEvent.setup();
        renderPage("email");
        await user.click(screen.getByRole("button", { name: "Replace SMTP password" }));
        await user.type(screen.getByLabelText("SMTP password"), "oops");

        // when
        await user.click(screen.getByRole("button", { name: "Keep saved value for SMTP password" }));

        // then
        expect(screen.getByRole("button", { name: "Replace SMTP password" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: SAVE_EMAIL })).toBeDisabled();
    });

    it("shows the fields that belong to the chosen email provider", async () => {
        // given
        const user = userEvent.setup();
        renderPage("email");
        const email = screen.getByRole("region", { name: "Email" });
        const smtpHostShownBefore = within(email).queryByRole("textbox", { name: "SMTP host" }) !== null;

        // when
        await user.selectOptions(within(email).getByRole("combobox", { name: "Provider" }), "cloudflare");

        // then
        expect(smtpHostShownBefore).toBe(true);
        expect(within(email).queryByRole("textbox", { name: "SMTP host" })).not.toBeInTheDocument();
        expect(within(email).getByRole("textbox", { name: "Account ID" })).toBeInTheDocument();
        expect(within(email).getByLabelText("API token")).toHaveAttribute("type", "password");
    });

    it("shows the Turnstile keys only while Turnstile is on", async () => {
        // given
        const user = userEvent.setup();
        renderPage("turnstile");
        const turnstile = screen.getByRole("region", { name: "Turnstile" });
        const keyBefore = within(turnstile).queryByRole("textbox", { name: "Site key" });

        // when
        await user.click(within(turnstile).getByRole("switch", { name: "Turnstile check" }));

        // then
        expect(keyBefore).not.toBeInTheDocument();
        expect(within(turnstile).getByRole("textbox", { name: "Site key" })).toBeInTheDocument();
        expect(within(turnstile).getByLabelText("Secret key")).toBeInTheDocument();
    });

    it("offers the test email inside the email section only", () => {
        // when
        renderPage("email");

        // then
        expect(
            within(screen.getByRole("region", { name: "Email" })).getByRole("button", { name: "Send test email" }),
        ).toBeInTheDocument();
    });

    it("saves the email settings on screen before sending a test email", async () => {
        // given
        const update = makeMutationState({ mutate: vi.fn(succeed) });
        const test = makeMutationState();
        useUpdateAdminSettings.mockReturnValue(update);
        useSendTestEmail.mockReturnValue(test);
        const user = userEvent.setup();
        renderPage("email");

        // when
        const host = screen.getByRole("textbox", { name: "SMTP host" });
        await user.clear(host);
        await user.type(host, "127.0.0.1");
        await user.click(screen.getByRole("button", { name: "Save and send test email" }));

        // then
        expect(update.mutate).toHaveBeenCalledWith({ settings: { smtp_host: "127.0.0.1" } }, expect.anything());
        expect(test.mutate).toHaveBeenCalledOnce();
    });

    it("clears the edits once the section is saved", async () => {
        // given
        const update = makeMutationState({ mutate: vi.fn(succeed) });
        useUpdateAdminSettings.mockReturnValue(update);
        const user = userEvent.setup();
        renderPage("email");

        // when
        await user.click(screen.getByRole("button", { name: "Replace SMTP password" }));
        await user.type(screen.getByLabelText("SMTP password"), "new-secret");
        await user.click(screen.getByRole("button", { name: SAVE_EMAIL }));

        // then
        expect(screen.getByRole("button", { name: "Replace SMTP password" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: SAVE_EMAIL })).toBeDisabled();
    });

    it("says the settings were saved", () => {
        // given
        useUpdateAdminSettings.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Settings saved.");
    });

    it("holds the form while the settings save", () => {
        // given
        useUpdateAdminSettings.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("textbox", { name: "Site name" })).toBeDisabled();
        expect(screen.getByRole("button", { name: SAVE_GENERAL })).toBeDisabled();
    });

    it("shows the server's reason when the settings were refused, and moves focus to it", () => {
        // given
        const error = makeApiError({ status: 400, message: "invalid setting value: smtp_port must be a whole number" });
        useUpdateAdminSettings.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderPage("email");

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("smtp_port must be a whole number");
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
