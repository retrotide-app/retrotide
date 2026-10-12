import { useQueryClient } from "@tanstack/react-query";
import { renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useContext } from "react";
import { useLocation, useParams } from "react-router";
import { describe, expect, it, vi } from "vitest";
import { AuthContext } from "../context/authContextValue";
import { OrgContext } from "../context/orgContextValue";
import { SiteInfoContext } from "../context/siteInfoContextValue";
import { ThemeContext } from "../context/themeContextValue";
import { makeOrganisation, makeSessionUser } from "./fixtures";
import { createTestQueryClient, providerWrapper, renderWithProviders } from "./render";

function provided<T>(value: T | null, name: string): T {
    if (value === null) {
        throw new Error(`${name} was not provided by the harness`);
    }

    return value;
}

function ContextProbe() {
    const siteInfo = provided(useContext(SiteInfoContext), "SiteInfoContext");
    const auth = provided(useContext(AuthContext), "AuthContext");
    const theme = provided(useContext(ThemeContext), "ThemeContext");
    const location = useLocation();

    return (
        <section>
            <p>{`Site: ${siteInfo.siteInfo?.site_name ?? "not loaded"}`}</p>
            <p>{siteInfo.loading ? "Site loading" : "Site loaded"}</p>
            <p>{auth.user === null ? "Signed out" : `Signed in as ${auth.user.display_name}`}</p>
            <p>{`Permissions: ${auth.permissions.join(", ") || "none"}`}</p>
            <p>{auth.loading ? "Loading" : "Loaded"}</p>
            <p>{`Theme: ${theme.preference}`}</p>
            <p>{`Location: ${location.pathname}${location.search}`}</p>
            <button type="button" onClick={() => theme.setPreference("dark")}>
                Use dark
            </button>
        </section>
    );
}

function OrgProbe() {
    const org = provided(useContext(OrgContext), "OrgContext");

    return (
        <section>
            <p>{`Organisations: ${org.organisations.map(listed => listed.name).join(", ") || "none"}`}</p>
            <p>{`Current: ${org.current?.name ?? "none"}`}</p>
            <p>{`Role: ${org.role ?? "none"}`}</p>
        </section>
    );
}

function ParamsProbe() {
    const { orgSlug } = useParams();

    return <p>{`Organisation: ${orgSlug}`}</p>;
}

function useProvidedContexts() {
    return {
        siteInfo: useContext(SiteInfoContext),
        auth: useContext(AuthContext),
        theme: useContext(ThemeContext),
    };
}

describe("createTestQueryClient", () => {
    it("turns off retries, caching, staleness and window-focus refetching", () => {
        // given
        const expected = {
            queries: { retry: false, gcTime: 0, staleTime: 0, refetchOnWindowFocus: false },
            mutations: { retry: false },
        };

        // when
        const defaults = createTestQueryClient().getDefaultOptions();

        // then
        expect(defaults).toEqual(expected);
    });
});

describe("renderWithProviders", () => {
    it("supplies RetroTide as the site name, no signed-in user and the system theme by default", () => {
        // given
        const ui = <ContextProbe />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("Site: RetroTide")).toBeInTheDocument();
        expect(screen.getByText("Site loaded")).toBeInTheDocument();
        expect(screen.getByText("Signed out")).toBeInTheDocument();
        expect(screen.getByText("Permissions: none")).toBeInTheDocument();
        expect(screen.getByText("Loaded")).toBeInTheDocument();
        expect(screen.getByText("Theme: system")).toBeInTheDocument();
    });

    it("grants the permissions option to the signed in user", () => {
        // given
        const options = { user: makeSessionUser(), permissions: ["view_admin_panel", "view_users"] as const };

        // when
        renderWithProviders(<ContextProbe />, options);

        // then
        expect(screen.getByText("Permissions: view_admin_panel, view_users")).toBeInTheDocument();
    });

    it("lets the siteInfoContext option stand in for a site whose info is still loading", () => {
        // given
        const siteInfoContext = { siteInfo: null, loading: true };

        // when
        renderWithProviders(<ContextProbe />, { siteInfoContext });

        // then
        expect(screen.getByText("Site: not loaded")).toBeInTheDocument();
        expect(screen.getByText("Site loading")).toBeInTheDocument();
    });

    it("starts the router at / when no route is given", () => {
        // given
        const ui = <ContextProbe />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("Location: /")).toBeInTheDocument();
    });

    it("signs in the user option", () => {
        // given
        const user = makeSessionUser();

        // when
        renderWithProviders(<ContextProbe />, { user });

        // then
        expect(screen.getByText("Signed in as Sam")).toBeInTheDocument();
    });

    it("merges the auth option into the auth context", () => {
        // given
        const auth = { loading: true };

        // when
        renderWithProviders(<ContextProbe />, { auth });

        // then
        expect(screen.getByText("Loading")).toBeInTheDocument();
        expect(screen.getByText("Signed out")).toBeInTheDocument();
    });

    it("lets the auth option's user win over the user option", () => {
        // given
        const user = makeSessionUser();
        const auth = { user: makeSessionUser({ display_name: "Robin" }) };

        // when
        renderWithProviders(<ContextProbe />, { user, auth });

        // then
        expect(screen.getByText("Signed in as Robin")).toBeInTheDocument();
    });

    it("merges the siteInfo option into the default site info", () => {
        // given
        const siteInfo = { site_name: "Harbour" };

        // when
        renderWithProviders(<ContextProbe />, { siteInfo });

        // then
        expect(screen.getByText("Site: Harbour")).toBeInTheDocument();
    });

    it("hands the theme option's preference and setter to the component", async () => {
        // given
        const setPreference = vi.fn();
        const viewer = userEvent.setup();
        renderWithProviders(<ContextProbe />, { theme: { preference: "light", setPreference } });

        // when
        await viewer.click(screen.getByRole("button", { name: "Use dark" }));

        // then
        expect(screen.getByText("Theme: light")).toBeInTheDocument();
        expect(setPreference).toHaveBeenCalledWith("dark");
    });

    it("supplies no organisation by default", () => {
        // given
        const ui = <OrgProbe />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("Organisations: none")).toBeInTheDocument();
        expect(screen.getByText("Current: none")).toBeInTheDocument();
        expect(screen.getByText("Role: none")).toBeInTheDocument();
    });

    it("makes the first organisation of the org option current, with the role it lists", () => {
        // given
        const organisations = [
            makeOrganisation({ name: "Harbour", role: "admin" }),
            makeOrganisation({ id: "o2", name: "Lighthouse" }),
        ];

        // when
        renderWithProviders(<OrgProbe />, { org: { organisations } });

        // then
        expect(screen.getByText("Organisations: Harbour, Lighthouse")).toBeInTheDocument();
        expect(screen.getByText("Current: Harbour")).toBeInTheDocument();
        expect(screen.getByText("Role: admin")).toBeInTheDocument();
    });

    it("lets the org option name the current organisation and its role", () => {
        // given
        const lighthouse = makeOrganisation({ id: "o2", name: "Lighthouse", role: "member" });
        const org = { organisations: [makeOrganisation(), lighthouse], current: lighthouse };

        // when
        renderWithProviders(<OrgProbe />, { org });

        // then
        expect(screen.getByText("Current: Lighthouse")).toBeInTheDocument();
        expect(screen.getByText("Role: member")).toBeInTheDocument();
    });

    it("starts the router at the route option", () => {
        // given
        const route = "/settings?tab=profile";

        // when
        renderWithProviders(<ContextProbe />, { route });

        // then
        expect(screen.getByText("Location: /settings?tab=profile")).toBeInTheDocument();
    });

    it("exposes the params of the path option to the component", () => {
        // given
        const options = { route: "/o/acme", path: "/o/:orgSlug" };

        // when
        renderWithProviders(<ParamsProbe />, options);

        // then
        expect(screen.getByText("Organisation: acme")).toBeInTheDocument();
    });

    it("returns the query client the test passed in", () => {
        // given
        const queryClient = createTestQueryClient();

        // when
        const result = renderWithProviders(<ContextProbe />, { queryClient });

        // then
        expect(result.queryClient).toBe(queryClient);
    });

    it("creates a fresh test query client for each render when none is passed", () => {
        // given
        const first = renderWithProviders(<ContextProbe />);

        // when
        const second = renderWithProviders(<ContextProbe />);

        // then
        expect(second.queryClient).not.toBe(first.queryClient);
        expect(second.queryClient.getDefaultOptions().queries?.retry).toBe(false);
    });

    it("wraps the next element in the same providers on rerender", () => {
        // given
        const { rerender } = renderWithProviders(<p>First</p>, { siteInfo: { site_name: "Harbour" }, route: "/later" });

        // when
        rerender(<ContextProbe />);

        // then
        expect(screen.queryByText("First")).not.toBeInTheDocument();
        expect(screen.getByText("Site: Harbour")).toBeInTheDocument();
        expect(screen.getByText("Location: /later")).toBeInTheDocument();
    });
});

describe("providerWrapper", () => {
    it("supplies the site info, auth and theme contexts to a hook", () => {
        // given
        const wrapper = providerWrapper({
            user: makeSessionUser({ display_name: "Robin" }),
            siteInfo: { site_name: "Harbour" },
            theme: { preference: "dark" },
        });

        // when
        const { result } = renderHook(useProvidedContexts, { wrapper });

        // then
        expect(result.current.siteInfo?.siteInfo?.site_name).toBe("Harbour");
        expect(result.current.auth?.user?.display_name).toBe("Robin");
        expect(result.current.theme?.preference).toBe("dark");
    });

    it("gives the default organisation context a mock chooser", () => {
        // given
        const wrapper = providerWrapper();

        // when
        const { result } = renderHook(() => useContext(OrgContext), { wrapper });

        // then
        expect(vi.isMockFunction(result.current?.selectOrganisation)).toBe(true);
    });

    it("gives the default theme a mock setter", () => {
        // given
        const wrapper = providerWrapper();

        // when
        const { result } = renderHook(useProvidedContexts, { wrapper });

        // then
        expect(vi.isMockFunction(result.current.theme?.setPreference)).toBe(true);
    });

    it("places the hook at the route option", () => {
        // given
        const wrapper = providerWrapper({ route: "/o/acme/settings" });

        // when
        const { result } = renderHook(useLocation, { wrapper });

        // then
        expect(result.current.pathname).toBe("/o/acme/settings");
    });

    it("uses the query client the test passed in", () => {
        // given
        const queryClient = createTestQueryClient();

        // when
        const { result } = renderHook(useQueryClient, { wrapper: providerWrapper({ queryClient }) });

        // then
        expect(result.current).toBe(queryClient);
    });

    it("keeps one query client across rerenders of the hook", () => {
        // given
        const { result, rerender } = renderHook(useQueryClient, { wrapper: providerWrapper() });
        const first = result.current;

        // when
        rerender();

        // then
        expect(result.current).toBe(first);
    });
});
