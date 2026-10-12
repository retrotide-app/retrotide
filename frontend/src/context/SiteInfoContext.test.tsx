import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSiteInfoState } from "../hooks/useSiteInfo";
import { makeApiError, makeQueryState, makeSiteInfo } from "../test-utils/fixtures";
import { renderWithProviders } from "../test-utils/render";
import { SiteInfoProvider } from "./SiteInfoContext";

const { useSiteInfoQuery } = vi.hoisted(() => ({ useSiteInfoQuery: vi.fn() }));

vi.mock("../hooks/queries/siteInfo", () => ({ useSiteInfoQuery }));

function Probe() {
    const { siteInfo, loading, error } = useSiteInfoState();

    return (
        <div>
            <p>{`site: ${siteInfo?.site_name ?? "none"}`}</p>
            <p>{`mode: ${siteInfo?.deployment_mode ?? "none"}`}</p>
            <p>{`loading: ${String(loading)}`}</p>
            <p>{`error: ${error?.message ?? "none"}`}</p>
        </div>
    );
}

function renderProvider() {
    return renderWithProviders(
        <SiteInfoProvider>
            <Probe />
        </SiteInfoProvider>,
        { siteInfo: { site_name: "Outer Harness" } },
    );
}

beforeEach(() => {
    useSiteInfoQuery.mockReturnValue(makeQueryState({ data: makeSiteInfo() }));
});

describe("SiteInfoProvider", () => {
    it("hands down the site info the server answered with, not the harness's", () => {
        // given
        useSiteInfoQuery.mockReturnValue(
            makeQueryState({ data: makeSiteInfo({ site_name: "Harbour", deployment_mode: "self_hosted" }) }),
        );

        // when
        renderProvider();

        // then
        expect(screen.getByText("site: Harbour")).toBeInTheDocument();
        expect(screen.getByText("mode: self_hosted")).toBeInTheDocument();
        expect(screen.queryByText("site: Outer Harness")).not.toBeInTheDocument();
    });

    it("hands down no site info while it is loading, rather than a guess", () => {
        // given
        useSiteInfoQuery.mockReturnValue(makeQueryState({ loading: true }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("site: none")).toBeInTheDocument();
        expect(screen.getByText("loading: true")).toBeInTheDocument();
    });

    it("hands down the server's error when the site info could not be read", () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to read the site info" });
        useSiteInfoQuery.mockReturnValue(makeQueryState({ error: failure }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("site: none")).toBeInTheDocument();
        expect(screen.getByText("error: failed to read the site info")).toBeInTheDocument();
    });
});
