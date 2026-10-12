import { useCallback, useEffect, useRef, useState } from "react";
import { reportClientError } from "../api/telemetry";
import { mountTurnstile, type TurnstileTheme, type TurnstileWidgetHandle } from "../platform/turnstile";
import type { ThemePreference } from "../types/app";
import { useSiteInfo } from "./useSiteInfo";
import { useTheme } from "./useTheme";

export interface TurnstileControl {
    enabled: boolean;
    token: string;
    error: string | null;
    attach: (node: HTMLDivElement | null) => void;
    reset: () => void;
}

export const TURNSTILE_MISSING_SITE_KEY =
    "Verification is switched on, but this site has no Turnstile site key. Ask the site admin to add one.";
export const TURNSTILE_LOAD_FAILED =
    "The verification check could not load. Check your connection, then reload the page.";
export const TURNSTILE_CHECK_FAILED = "The verification check failed. Try it again.";

const THEMES: Readonly<Record<ThemePreference, TurnstileTheme>> = { system: "auto", light: "light", dark: "dark" };

export function useTurnstile(): TurnstileControl {
    const { turnstile_enabled, turnstile_site_key } = useSiteInfo();
    const { preference } = useTheme();
    const [container, setContainer] = useState<HTMLDivElement | null>(null);
    const [token, setToken] = useState("");
    const [failure, setFailure] = useState<string | null>(null);
    const widget = useRef<TurnstileWidgetHandle | null>(null);

    const siteKey = turnstile_site_key.trim();
    const theme = THEMES[preference];

    useEffect(() => {
        if (!turnstile_enabled || siteKey === "" || container === null) {
            return;
        }

        let active = true;

        mountTurnstile(container, {
            siteKey,
            theme,
            onToken: next => {
                setToken(next);
                setFailure(null);
            },
            onExpire: () => {
                setToken("");
            },
            onError: () => {
                setToken("");
                setFailure(TURNSTILE_CHECK_FAILED);
            },
        })
            .then(handle => {
                if (!active) {
                    handle.remove();
                    return;
                }

                widget.current = handle;
            })
            .catch((error: unknown) => {
                reportClientError(error, { source: "turnstile" });

                if (active) {
                    setFailure(TURNSTILE_LOAD_FAILED);
                }
            });

        return () => {
            active = false;
            widget.current?.remove();
            widget.current = null;
        };
    }, [turnstile_enabled, siteKey, theme, container]);

    const reset = useCallback(() => {
        setToken("");
        widget.current?.reset();
    }, []);

    const error = turnstile_enabled && siteKey === "" ? TURNSTILE_MISSING_SITE_KEY : failure;

    return { enabled: turnstile_enabled, token, error, attach: setContainer, reset };
}
