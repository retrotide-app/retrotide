import { describe, expect, it } from "vitest";
import {
    changedSettings,
    isSecretSet,
    isSettingShown,
    SECRET_MASK,
    SITE_SETTING_GROUPS,
    type SiteSettingField,
    settingOptions,
    settingValue,
} from "./siteSettings";

const serverSettingKeys = [
    "base_url",
    "site_name",
    "log_level",
    "otlp_endpoint",
    "pyroscope_url",
    "max_body_size",
    "maintenance_mode",
    "metrics_token",
    "valkey_url",
    "cache_in_memory_max_mb",
    "site_mode",
    "min_password_length",
    "session_duration_days",
    "allow_password_sign_in",
    "turnstile_enabled",
    "turnstile_site_key",
    "turnstile_secret_key",
    "dronebl_enabled",
    "dronebl_allowlist",
    "dronebl_ignored_classes",
    "email_provider",
    "smtp_host",
    "smtp_port",
    "smtp_from",
    "smtp_username",
    "smtp_password",
    "cloudflare_account_id",
    "cloudflare_api_token",
    "cloudflare_email_from",
    "google_client_id",
    "google_client_secret",
    "microsoft_client_id",
    "microsoft_client_secret",
    "microsoft_tenant_id",
    "microsoft_personal_accounts",
];

const serverSecretKeys = [
    "metrics_token",
    "valkey_url",
    "turnstile_secret_key",
    "smtp_password",
    "cloudflare_api_token",
    "google_client_secret",
    "microsoft_client_secret",
];

function allFields(): SiteSettingField[] {
    return SITE_SETTING_GROUPS.flatMap(group => group.fields);
}

describe("SITE_SETTING_GROUPS", () => {
    it("offers every setting the server keeps, once each", () => {
        // given
        const fields = allFields();

        // when
        const keys = fields.map(field => field.key);

        // then
        expect(new Set(keys).size).toBe(keys.length);
        expect([...keys].sort()).toEqual([...serverSettingKeys].sort());
    });

    it("treats exactly the settings the server masks as secrets", () => {
        // given
        const fields = allFields();

        // when
        const secrets = fields.filter(field => field.kind === "secret").map(field => field.key);

        // then
        expect([...secrets].sort()).toEqual([...serverSecretKeys].sort());
    });

    it("offers exactly the site modes the server accepts", () => {
        // given
        const siteMode = allFields().find(field => field.key === "site_mode");

        // when
        const values = siteMode?.options?.map(option => option.value);

        // then
        expect(values).toEqual(["public", "private"]);
    });

    it("offers the password switch with the accounts and puts microsoft right after google", () => {
        // given
        const groups = SITE_SETTING_GROUPS;

        // when
        const accounts = groups.find(group => group.id === "accounts");
        const microsoft = groups.find(group => group.id === "microsoft");
        const ids = groups.map(group => group.id);

        // then
        expect(accounts?.fields.find(field => field.key === "allow_password_sign_in")?.kind).toBe("toggle");
        expect(ids.indexOf("microsoft")).toBe(ids.indexOf("google") + 1);
        expect(microsoft?.fields.map(field => field.kind)).toEqual(["text", "secret", "text", "toggle"]);
    });

    it("offers exactly the email providers the server accepts", () => {
        // given
        const provider = allFields().find(field => field.key === "email_provider");

        // when
        const values = provider?.options?.map(option => option.value);

        // then
        expect(values).toEqual(["smtp", "cloudflare"]);
    });
});

describe("settingValue", () => {
    it("prefers the value being edited over the saved one", () => {
        // given
        const saved = { site_name: "RetroTide" };
        const draft = { site_name: "Retro" };

        // when
        const value = settingValue(saved, draft, "site_name");

        // then
        expect(value).toBe("Retro");
    });

    it("falls back to the saved value, then to empty", () => {
        // given
        const saved = { site_name: "RetroTide" };
        const draft = {};

        // when
        const values = [settingValue(saved, draft, "site_name"), settingValue(saved, draft, "smtp_host")];

        // then
        expect(values).toEqual(["RetroTide", ""]);
    });
});

describe("isSettingShown", () => {
    it("always shows a field with no condition", () => {
        // given
        const field: SiteSettingField = { key: "site_name", label: "Site name", kind: "text" };

        // when
        const shown = isSettingShown(field, {}, {});

        // then
        expect(shown).toBe(true);
    });

    it("shows a conditional field only while its condition holds, edits included", () => {
        // given
        const field: SiteSettingField = {
            key: "smtp_host",
            label: "SMTP host",
            kind: "text",
            shownWhen: { key: "email_provider", value: "smtp" },
        };
        const saved = { email_provider: "smtp" };

        // when
        const whileSaved = isSettingShown(field, saved, {});
        const afterSwitching = isSettingShown(field, saved, { email_provider: "cloudflare" });

        // then
        expect(whileSaved).toBe(true);
        expect(afterSwitching).toBe(false);
    });
});

describe("isSecretSet", () => {
    it("reads the server's mask as a secret that is set", () => {
        // given
        const values = [SECRET_MASK, "", "hunter2"];

        // when
        const set = values.map(value => isSecretSet(value));

        // then
        expect(set).toEqual([true, false, false]);
    });
});

describe("settingOptions", () => {
    const logLevel: SiteSettingField = {
        key: "log_level",
        label: "Log level",
        kind: "choice",
        options: [
            { value: "info", label: "Info" },
            { value: "error", label: "Error" },
        ],
    };

    it("offers the listed options when the current value is one of them", () => {
        // given
        const current = "info";

        // when
        const options = settingOptions(logLevel, current);

        // then
        expect(options.map(option => option.value)).toEqual(["info", "error"]);
    });

    it("keeps a current value the list does not know, so saving never changes it silently", () => {
        // given
        const current = "fatal";

        // when
        const options = settingOptions(logLevel, current);

        // then
        expect(options).toEqual([
            { value: "info", label: "Info" },
            { value: "error", label: "Error" },
            { value: "fatal", label: "fatal" },
        ]);
    });
});

describe("changedSettings", () => {
    it("keeps only the edits that differ from the saved values", () => {
        // given
        const saved = { site_name: "RetroTide", smtp_port: "25", smtp_password: SECRET_MASK };
        const draft = { site_name: "RetroTide", smtp_port: "587", smtp_password: "" };

        // when
        const changed = changedSettings(saved, draft);

        // then
        expect(changed).toEqual({ smtp_port: "587", smtp_password: "" });
    });

    it("sends nothing when nothing changed", () => {
        // given
        const saved = { site_name: "RetroTide" };
        const draft = {};

        // when
        const changed = changedSettings(saved, draft);

        // then
        expect(changed).toEqual({});
    });
});
