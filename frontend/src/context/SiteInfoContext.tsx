import { type PropsWithChildren, useMemo } from "react";
import { useSiteInfoQuery } from "../hooks/queries/siteInfo";
import { SiteInfoContext, type SiteInfoContextValue } from "./siteInfoContextValue";

export function SiteInfoProvider({ children }: PropsWithChildren) {
    const { data, loading, error, refetch } = useSiteInfoQuery();
    const siteInfo = data ?? null;

    const value = useMemo<SiteInfoContextValue>(
        () => ({ siteInfo, loading, error, retry: refetch }),
        [siteInfo, loading, error, refetch],
    );

    return <SiteInfoContext.Provider value={value}>{children}</SiteInfoContext.Provider>;
}
