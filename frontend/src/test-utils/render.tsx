import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, type RenderOptions, type RenderResult } from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { AuthContext, type AuthContextValue } from "../context/authContextValue";
import { OrgContext, type OrgContextValue } from "../context/orgContextValue";
import { SiteInfoContext, type SiteInfoContextValue } from "../context/siteInfoContextValue";
import { ThemeContext, type ThemeContextValue } from "../context/themeContextValue";
import type { StaffPermission } from "../domain/permissions";
import type { SessionUser, SiteInfo } from "../types/app";
import { makeAuthContext, makeOrgContext, makeSiteInfo, makeSiteInfoContext, makeThemeContext } from "./fixtures";

export interface ProviderOptions {
    user?: SessionUser | null;
    permissions?: readonly StaffPermission[];
    auth?: Partial<AuthContextValue>;
    siteInfo?: Partial<SiteInfo>;
    siteInfoContext?: Partial<SiteInfoContextValue>;
    org?: Partial<OrgContextValue>;
    theme?: Partial<ThemeContextValue>;
    queryClient?: QueryClient;
    route?: string;
    routeState?: unknown;
    path?: string;
}

export function createTestQueryClient(): QueryClient {
    return new QueryClient({
        defaultOptions: {
            queries: { retry: false, gcTime: 0, staleTime: 0, refetchOnWindowFocus: false },
            mutations: { retry: false },
        },
    });
}

function wrap(children: ReactNode, options: ProviderOptions, queryClient: QueryClient): ReactElement {
    const auth = makeAuthContext({
        user: options.user ?? null,
        permissions: options.permissions ?? [],
        ...options.auth,
    });
    const siteInfo = makeSiteInfoContext({ siteInfo: makeSiteInfo(options.siteInfo), ...options.siteInfoContext });
    const org = makeOrgContext(options.org);
    const theme = makeThemeContext(options.theme);

    const route = options.route ?? "/";
    const entry = options.routeState === undefined ? route : { pathname: route, state: options.routeState };

    const routed = options.path ? (
        <Routes>
            <Route path={options.path} element={children} />
        </Routes>
    ) : (
        children
    );

    return (
        <QueryClientProvider client={queryClient}>
            <MemoryRouter initialEntries={[entry]}>
                <SiteInfoContext.Provider value={siteInfo}>
                    <AuthContext.Provider value={auth}>
                        <OrgContext.Provider value={org}>
                            <ThemeContext.Provider value={theme}>{routed}</ThemeContext.Provider>
                        </OrgContext.Provider>
                    </AuthContext.Provider>
                </SiteInfoContext.Provider>
            </MemoryRouter>
        </QueryClientProvider>
    );
}

export interface RenderWithProvidersResult extends RenderResult {
    queryClient: QueryClient;
}

export function renderWithProviders(
    ui: ReactElement,
    options: ProviderOptions = {},
    renderOptions: Omit<RenderOptions, "wrapper"> = {},
): RenderWithProvidersResult {
    const queryClient = options.queryClient ?? createTestQueryClient();
    const result = render(wrap(ui, options, queryClient), renderOptions);

    return {
        ...result,
        queryClient,
        rerender: (next: ReactNode) => result.rerender(wrap(next, options, queryClient)),
    };
}

export function providerWrapper(
    options: ProviderOptions = {},
): ({ children }: { children: ReactNode }) => ReactElement {
    const queryClient = options.queryClient ?? createTestQueryClient();

    return ({ children }: { children: ReactNode }) => wrap(children, options, queryClient);
}
