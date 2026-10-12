import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TurnstileApi, TurnstileMount } from "./turnstile";

type TurnstileModule = typeof import("./turnstile");

interface TurnstileWindow {
    turnstile?: TurnstileApi;
}

function fakeApi(widgetId: string | null = "widget-1") {
    return {
        render: vi.fn<TurnstileApi["render"]>(() => widgetId),
        reset: vi.fn<TurnstileApi["reset"]>(),
        remove: vi.fn<TurnstileApi["remove"]>(),
    };
}

function mountOptions(): TurnstileMount {
    return { siteKey: "site-key-1", theme: "dark", onToken: vi.fn(), onExpire: vi.fn(), onError: vi.fn() };
}

function turnstileScripts(url: string): HTMLScriptElement[] {
    return Array.from(document.head.querySelectorAll<HTMLScriptElement>("script")).filter(script => script.src === url);
}

async function freshModule(): Promise<TurnstileModule> {
    vi.resetModules();

    return import("./turnstile");
}

function finishLoading(api: TurnstileApi | undefined, url: string): void {
    (window as TurnstileWindow).turnstile = api;

    const [script] = turnstileScripts(url);
    script.dispatchEvent(new Event("load"));
}

let container: HTMLDivElement;

beforeEach(() => {
    container = document.createElement("div");
});

afterEach(() => {
    delete (window as TurnstileWindow).turnstile;

    for (const script of Array.from(document.head.querySelectorAll("script"))) {
        script.remove();
    }
});

describe("mountTurnstile", () => {
    it("loads Cloudflare's script once, for explicit rendering", async () => {
        // given
        const turnstile = await freshModule();

        // when
        const mounting = turnstile.mountTurnstile(container, mountOptions());
        finishLoading(fakeApi(), turnstile.TURNSTILE_SCRIPT_URL);
        await mounting;

        // then
        expect(turnstile.TURNSTILE_SCRIPT_URL).toBe(
            "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit",
        );
        expect(turnstileScripts(turnstile.TURNSTILE_SCRIPT_URL)).toHaveLength(1);
    });

    it("renders the widget into the container with the site key, the theme and the callbacks", async () => {
        // given
        const turnstile = await freshModule();
        const api = fakeApi();
        const options = mountOptions();

        // when
        const mounting = turnstile.mountTurnstile(container, options);
        finishLoading(api, turnstile.TURNSTILE_SCRIPT_URL);
        await mounting;

        // then
        expect(api.render).toHaveBeenCalledWith(container, {
            sitekey: "site-key-1",
            theme: "dark",
            callback: options.onToken,
            "expired-callback": options.onExpire,
            "error-callback": options.onError,
        });
    });

    it("resets and removes the widget it rendered", async () => {
        // given
        const turnstile = await freshModule();
        const api = fakeApi("widget-7");
        const mounting = turnstile.mountTurnstile(container, mountOptions());
        finishLoading(api, turnstile.TURNSTILE_SCRIPT_URL);
        const widget = await mounting;

        // when
        widget.reset();
        widget.remove();

        // then
        expect(api.reset).toHaveBeenCalledWith("widget-7");
        expect(api.remove).toHaveBeenCalledWith("widget-7");
    });

    it("shares one script between two widgets that mount together", async () => {
        // given
        const turnstile = await freshModule();
        const api = fakeApi();

        // when
        const first = turnstile.mountTurnstile(container, mountOptions());
        const second = turnstile.mountTurnstile(document.createElement("div"), mountOptions());
        finishLoading(api, turnstile.TURNSTILE_SCRIPT_URL);
        await Promise.all([first, second]);

        // then
        expect(turnstileScripts(turnstile.TURNSTILE_SCRIPT_URL)).toHaveLength(1);
        expect(api.render).toHaveBeenCalledTimes(2);
    });

    it("uses a Turnstile the page already has without loading the script again", async () => {
        // given
        const turnstile = await freshModule();
        const api = fakeApi();
        (window as TurnstileWindow).turnstile = api;

        // when
        await turnstile.mountTurnstile(container, mountOptions());

        // then
        expect(turnstileScripts(turnstile.TURNSTILE_SCRIPT_URL)).toHaveLength(0);
        expect(api.render).toHaveBeenCalledOnce();
    });

    it("fails when the script cannot be fetched, and tries again on the next mount", async () => {
        // given
        const turnstile = await freshModule();
        const failed = turnstile.mountTurnstile(container, mountOptions());
        turnstileScripts(turnstile.TURNSTILE_SCRIPT_URL)[0].dispatchEvent(new Event("error"));
        await expect(failed).rejects.toThrow("could not be loaded");

        // when
        const retried = turnstile.mountTurnstile(container, mountOptions());
        finishLoading(fakeApi(), turnstile.TURNSTILE_SCRIPT_URL);

        // then
        await expect(retried).resolves.toEqual({ reset: expect.any(Function), remove: expect.any(Function) });
        expect(turnstileScripts(turnstile.TURNSTILE_SCRIPT_URL)).toHaveLength(1);
    });

    it("fails when the script defines no turnstile global", async () => {
        // given
        const turnstile = await freshModule();

        // when
        const mounting = turnstile.mountTurnstile(container, mountOptions());
        finishLoading(undefined, turnstile.TURNSTILE_SCRIPT_URL);

        // then
        await expect(mounting).rejects.toThrow("defined no turnstile global");
    });

    it("fails when Turnstile renders no widget", async () => {
        // given
        const turnstile = await freshModule();
        (window as TurnstileWindow).turnstile = fakeApi(null);

        // when
        const mounting = turnstile.mountTurnstile(container, mountOptions());

        // then
        await expect(mounting).rejects.toThrow("did not render its widget");
    });
});
