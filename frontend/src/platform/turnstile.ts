export const TURNSTILE_SCRIPT_URL = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

export type TurnstileTheme = "auto" | "light" | "dark";

export interface TurnstileMount {
    siteKey: string;
    theme: TurnstileTheme;
    onToken: (token: string) => void;
    onExpire: () => void;
    onError: () => void;
}

export interface TurnstileWidgetHandle {
    reset: () => void;
    remove: () => void;
}

export interface TurnstileApi {
    render: (container: HTMLElement, params: Record<string, unknown>) => string | null | undefined;
    reset: (widgetId: string) => void;
    remove: (widgetId: string) => void;
}

interface TurnstileWindow {
    turnstile?: TurnstileApi;
}

let loading: Promise<TurnstileApi> | null = null;

function loadedApi(): TurnstileApi | undefined {
    return (window as TurnstileWindow).turnstile;
}

function loadTurnstile(): Promise<TurnstileApi> {
    const ready = loadedApi();

    if (ready) {
        return Promise.resolve(ready);
    }

    if (loading) {
        return loading;
    }

    loading = new Promise<TurnstileApi>((resolve, reject) => {
        const script = document.createElement("script");
        script.src = TURNSTILE_SCRIPT_URL;
        script.async = true;

        script.addEventListener("load", () => {
            const api = loadedApi();

            if (!api) {
                loading = null;
                reject(new Error("the Turnstile script loaded but defined no turnstile global"));
                return;
            }

            resolve(api);
        });

        script.addEventListener("error", () => {
            loading = null;
            script.remove();
            reject(new Error(`the Turnstile script could not be loaded from ${TURNSTILE_SCRIPT_URL}`));
        });

        document.head.append(script);
    });

    return loading;
}

export async function mountTurnstile(container: HTMLElement, mount: TurnstileMount): Promise<TurnstileWidgetHandle> {
    const api = await loadTurnstile();

    const widgetId = api.render(container, {
        sitekey: mount.siteKey,
        theme: mount.theme,
        callback: mount.onToken,
        "expired-callback": mount.onExpire,
        "error-callback": mount.onError,
    });

    if (typeof widgetId !== "string" || widgetId === "") {
        throw new Error("Turnstile did not render its widget");
    }

    return {
        reset: () => {
            api.reset(widgetId);
        },
        remove: () => {
            api.remove(widgetId);
        },
    };
}
