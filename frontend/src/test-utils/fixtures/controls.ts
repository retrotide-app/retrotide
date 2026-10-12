import { vi } from "vitest";
import type { ProviderSignIn } from "../../hooks/useProviderSignIn";
import type { TurnstileControl } from "../../hooks/useTurnstile";

export function makeProviderSignIn(overrides: Partial<ProviderSignIn> = {}): ProviderSignIn {
    return { start: vi.fn(), provider: null, pending: false, error: null, reset: vi.fn(), ...overrides };
}

export function makeTurnstileControl(overrides: Partial<TurnstileControl> = {}): TurnstileControl {
    return { enabled: false, token: "", error: null, attach: vi.fn(), reset: vi.fn(), ...overrides };
}
