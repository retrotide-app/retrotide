import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TurnstileMount } from "../platform/turnstile";
import { providerWrapper, type ProviderOptions } from "../test-utils/render";
import {
    TURNSTILE_CHECK_FAILED,
    TURNSTILE_LOAD_FAILED,
    TURNSTILE_MISSING_SITE_KEY,
    useTurnstile,
} from "./useTurnstile";

const { mountTurnstile, reportClientError } = vi.hoisted(() => ({
    mountTurnstile: vi.fn(),
    reportClientError: vi.fn(),
}));

vi.mock("../platform/turnstile", () => ({ mountTurnstile }));
vi.mock("../api/telemetry", () => ({ reportClientError }));

const ENABLED: ProviderOptions = { siteInfo: { turnstile_enabled: true, turnstile_site_key: "site-key-1" } };

function widgetHandle() {
    return { reset: vi.fn(), remove: vi.fn() };
}

function lastMount(): TurnstileMount {
    return mountTurnstile.mock.lastCall?.[1];
}

async function mounted(options: ProviderOptions = ENABLED) {
    const handle = widgetHandle();
    mountTurnstile.mockResolvedValue(handle);
    const rendered = renderHook(useTurnstile, { wrapper: providerWrapper(options) });
    const container = document.createElement("div");

    act(() => {
        rendered.result.current.attach(container);
    });
    await waitFor(() => expect(mountTurnstile).toHaveBeenCalled());
    await act(() => Promise.resolve());

    return { ...rendered, handle, container };
}

beforeEach(() => {
    mountTurnstile.mockReset();
    reportClientError.mockReset();
});

describe("useTurnstile while the site has it switched off", () => {
    it("reports itself off and mounts nothing", () => {
        // given
        const { result } = renderHook(useTurnstile, { wrapper: providerWrapper() });

        // when
        act(() => {
            result.current.attach(document.createElement("div"));
        });

        // then
        expect(result.current.enabled).toBe(false);
        expect(result.current.error).toBeNull();
        expect(mountTurnstile).not.toHaveBeenCalled();
    });
});

describe("useTurnstile while the site has it switched on", () => {
    it("mounts the widget into the container with the site key", async () => {
        // when
        const { result, container } = await mounted();

        // then
        expect(result.current.enabled).toBe(true);
        expect(mountTurnstile).toHaveBeenCalledWith(container, expect.objectContaining({ siteKey: "site-key-1" }));
    });

    it("follows the system theme until the viewer picks one", async () => {
        // when
        await mounted();

        // then
        expect(lastMount().theme).toBe("auto");
    });

    it("matches a theme the viewer picked", async () => {
        // when
        await mounted({ ...ENABLED, theme: { preference: "dark" } });

        // then
        expect(lastMount().theme).toBe("dark");
    });

    it("holds the token once the check passes", async () => {
        // given
        const { result } = await mounted();

        // when
        act(() => {
            lastMount().onToken("token-1");
        });

        // then
        expect(result.current.token).toBe("token-1");
    });

    it("drops the token when it expires", async () => {
        // given
        const { result } = await mounted();
        act(() => {
            lastMount().onToken("token-1");
        });

        // when
        act(() => {
            lastMount().onExpire();
        });

        // then
        expect(result.current.token).toBe("");
    });

    it("says the check failed when Turnstile reports an error, and clears it when a token arrives", async () => {
        // given
        const { result } = await mounted();
        act(() => {
            lastMount().onError();
        });
        expect(result.current.error).toBe(TURNSTILE_CHECK_FAILED);

        // when
        act(() => {
            lastMount().onToken("token-2");
        });

        // then
        expect(result.current.error).toBeNull();
        expect(result.current.token).toBe("token-2");
    });

    it("resets the widget and forgets the used token", async () => {
        // given
        const { result, handle } = await mounted();
        act(() => {
            lastMount().onToken("token-1");
        });
        expect(result.current.token).toBe("token-1");

        // when
        act(() => {
            result.current.reset();
        });

        // then
        expect(result.current.token).toBe("");
        expect(handle.reset).toHaveBeenCalledOnce();
    });

    it("removes the widget when the form goes away", async () => {
        // given
        const { unmount, handle } = await mounted();

        // when
        unmount();

        // then
        expect(handle.remove).toHaveBeenCalledOnce();
    });

    it("says the check could not load, and reports why, when the script fails", async () => {
        // given
        const failure = new Error("the Turnstile script could not be loaded");
        mountTurnstile.mockRejectedValue(failure);
        const { result } = renderHook(useTurnstile, { wrapper: providerWrapper(ENABLED) });

        // when
        act(() => {
            result.current.attach(document.createElement("div"));
        });

        // then
        await waitFor(() => expect(result.current.error).toBe(TURNSTILE_LOAD_FAILED));
        expect(reportClientError).toHaveBeenCalledWith(failure, { source: "turnstile" });
    });

    it("says the site key is missing rather than showing a widget that cannot work", () => {
        // given
        const options = { siteInfo: { turnstile_enabled: true, turnstile_site_key: "  " } };
        const { result } = renderHook(useTurnstile, { wrapper: providerWrapper(options) });

        // when
        act(() => {
            result.current.attach(document.createElement("div"));
        });

        // then
        expect(result.current.enabled).toBe(true);
        expect(result.current.error).toBe(TURNSTILE_MISSING_SITE_KEY);
        expect(mountTurnstile).not.toHaveBeenCalled();
    });
});
