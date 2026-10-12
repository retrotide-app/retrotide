export type SiteSettingKind = "text" | "secret" | "number" | "toggle" | "choice";

export interface SiteSettingOption {
    value: string;
    label: string;
}

export interface SiteSettingCondition {
    key: string;
    value: string;
}

export interface SiteSettingField {
    key: string;
    label: string;
    kind: SiteSettingKind;
    hint?: string;
    options?: readonly SiteSettingOption[];
    shownWhen?: SiteSettingCondition;
}

export interface SiteSettingGroup {
    id: string;
    title: string;
    description?: string;
    fields: readonly SiteSettingField[];
}

export type SiteSettingValues = Readonly<Record<string, string>>;

export const SECRET_MASK = "********";

export const TOGGLE_ON = "true";
export const TOGGLE_OFF = "false";

export const EMAIL_GROUP_ID = "email";

const smtpProvider: SiteSettingCondition = { key: "email_provider", value: "smtp" };
const cloudflareProvider: SiteSettingCondition = { key: "email_provider", value: "cloudflare" };
const turnstileOn: SiteSettingCondition = { key: "turnstile_enabled", value: TOGGLE_ON };
const droneBLOn: SiteSettingCondition = { key: "dronebl_enabled", value: TOGGLE_ON };

export const SITE_SETTING_GROUPS: readonly SiteSettingGroup[] = [
    {
        id: "general",
        title: "General",
        fields: [
            { key: "site_name", label: "Site name", kind: "text" },
            {
                key: "base_url",
                label: "Site address",
                kind: "text",
                hint: "The public address people use, such as https://retrotide.app. Email links start with it.",
            },
            {
                key: "maintenance_mode",
                label: "Maintenance mode",
                kind: "toggle",
                hint: "Only staff who manage settings can use the site while it is on.",
            },
            {
                key: "site_mode",
                label: "Site mode",
                kind: "choice",
                hint: "On a private site, everyone must sign in and only people you invite can join.",
                options: [
                    { value: "public", label: "Public" },
                    { value: "private", label: "Private" },
                ],
            },
        ],
    },
    {
        id: "accounts",
        title: "Accounts",
        fields: [
            { key: "min_password_length", label: "Minimum password length", kind: "number" },
            { key: "session_duration_days", label: "Days a sign-in lasts", kind: "number" },
            {
                key: "allow_password_sign_in",
                label: "Password sign-in",
                kind: "toggle",
                hint: "Turn off to sign in only with the providers below. You can turn it off only on a public site with a provider set up.",
            },
        ],
    },
    {
        id: "google",
        title: "Google sign-in",
        description: "Leave both empty to turn Google sign-in off.",
        fields: [
            { key: "google_client_id", label: "Client ID", kind: "text" },
            { key: "google_client_secret", label: "Client secret", kind: "secret" },
        ],
    },
    {
        id: "microsoft",
        title: "Microsoft sign-in",
        description:
            "Leave the client ID and secret empty to turn Microsoft sign-in off. Register <site address>/api/v1/auth/microsoft/callback as the redirect URI, and add the optional email and xms_edov claims to the ID token so accounts can be created and linked automatically.",
        fields: [
            { key: "microsoft_client_id", label: "Client ID", kind: "text" },
            { key: "microsoft_client_secret", label: "Client secret", kind: "secret" },
            {
                key: "microsoft_tenant_id",
                label: "Tenant ID",
                kind: "text",
                hint: "Leave empty to admit work accounts from any organisation. Set a directory (tenant) ID to admit only that organisation.",
            },
            {
                key: "microsoft_personal_accounts",
                label: "Personal Microsoft accounts",
                kind: "toggle",
                hint: "Allow outlook.com and other personal Microsoft accounts. Ignored when a tenant ID is set.",
            },
        ],
    },
    {
        id: "turnstile",
        title: "Turnstile",
        fields: [
            {
                key: "turnstile_enabled",
                label: "Turnstile check",
                kind: "toggle",
                hint: "Ask for a Cloudflare Turnstile check when people sign in, register or ask for a reset link.",
            },
            { key: "turnstile_site_key", label: "Site key", kind: "text", shownWhen: turnstileOn },
            { key: "turnstile_secret_key", label: "Secret key", kind: "secret", shownWhen: turnstileOn },
        ],
    },
    {
        id: "dronebl",
        title: "DroneBL",
        fields: [
            {
                key: "dronebl_enabled",
                label: "DroneBL blocking",
                kind: "toggle",
                hint: "Refuse visitors whose address is on the DroneBL blocklist. Signed-in people are never blocked.",
            },
            {
                key: "dronebl_allowlist",
                label: "Allowlist",
                kind: "text",
                hint: "Addresses or ranges that are never checked, separated by commas.",
                shownWhen: droneBLOn,
            },
            {
                key: "dronebl_ignored_classes",
                label: "Ignored listing classes",
                kind: "text",
                hint: "DroneBL class numbers that never block, separated by commas.",
                shownWhen: droneBLOn,
            },
        ],
    },
    {
        id: EMAIL_GROUP_ID,
        title: "Email",
        fields: [
            {
                key: "email_provider",
                label: "Provider",
                kind: "choice",
                options: [
                    { value: "smtp", label: "SMTP" },
                    { value: "cloudflare", label: "Cloudflare Email Service" },
                ],
            },
            { key: "smtp_host", label: "SMTP host", kind: "text", shownWhen: smtpProvider },
            { key: "smtp_port", label: "SMTP port", kind: "number", shownWhen: smtpProvider },
            { key: "smtp_from", label: "From address", kind: "text", shownWhen: smtpProvider },
            {
                key: "smtp_username",
                label: "SMTP username",
                kind: "text",
                hint: "Leave empty if the server needs no sign-in.",
                shownWhen: smtpProvider,
            },
            { key: "smtp_password", label: "SMTP password", kind: "secret", shownWhen: smtpProvider },
            { key: "cloudflare_account_id", label: "Account ID", kind: "text", shownWhen: cloudflareProvider },
            { key: "cloudflare_api_token", label: "API token", kind: "secret", shownWhen: cloudflareProvider },
            { key: "cloudflare_email_from", label: "From address", kind: "text", shownWhen: cloudflareProvider },
        ],
    },
    {
        id: "performance",
        title: "Limits and cache",
        fields: [
            { key: "max_body_size", label: "Largest request body in bytes", kind: "number" },
            {
                key: "valkey_url",
                label: "Valkey URL",
                kind: "secret",
                hint: "Leave empty to use the in-memory cache.",
            },
            { key: "cache_in_memory_max_mb", label: "In-memory cache size in MB", kind: "number" },
        ],
    },
    {
        id: "monitoring",
        title: "Logging and monitoring",
        fields: [
            {
                key: "log_level",
                label: "Log level",
                kind: "choice",
                options: [
                    { value: "trace", label: "Trace" },
                    { value: "debug", label: "Debug" },
                    { value: "info", label: "Info" },
                    { value: "warn", label: "Warn" },
                    { value: "error", label: "Error" },
                ],
            },
            {
                key: "otlp_endpoint",
                label: "OTLP endpoint",
                kind: "text",
                hint: "Where traces are sent. Leave empty to turn tracing off.",
            },
            {
                key: "pyroscope_url",
                label: "Pyroscope URL",
                kind: "text",
                hint: "Where profiles are sent. Leave empty to turn profiling off.",
            },
            {
                key: "metrics_token",
                label: "Metrics token",
                kind: "secret",
                hint: "The token that /metrics asks for. Leave empty to leave /metrics open.",
            },
        ],
    },
];

export const SETTINGS_SECTION_PARAM = "section";

export function settingGroupFor(section: string | null): SiteSettingGroup {
    return SITE_SETTING_GROUPS.find(group => group.id === section) ?? SITE_SETTING_GROUPS[0];
}

export function settingValue(saved: SiteSettingValues, draft: SiteSettingValues, key: string): string {
    return draft[key] ?? saved[key] ?? "";
}

export function isSettingShown(field: SiteSettingField, saved: SiteSettingValues, draft: SiteSettingValues): boolean {
    if (field.shownWhen === undefined) {
        return true;
    }

    return settingValue(saved, draft, field.shownWhen.key) === field.shownWhen.value;
}

export function isSecretSet(value: string): boolean {
    return value === SECRET_MASK;
}

export function settingOptions(field: SiteSettingField, current: string): readonly SiteSettingOption[] {
    const options = field.options ?? [];

    if (current === "" || options.some(option => option.value === current)) {
        return options;
    }

    return [...options, { value: current, label: current }];
}

export function changedSettings(saved: SiteSettingValues, draft: SiteSettingValues): Record<string, string> {
    const changed: Record<string, string> = {};

    for (const [key, value] of Object.entries(draft)) {
        if (saved[key] === value) {
            continue;
        }

        changed[key] = value;
    }

    return changed;
}
