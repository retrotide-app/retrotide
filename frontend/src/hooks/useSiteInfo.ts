import { useContext } from "react";
import { SiteInfoContext, type SiteInfoContextValue } from "../context/siteInfoContextValue";
import type { SiteInfo } from "../types/app";

export function useSiteInfoState(): SiteInfoContextValue {
    const ctx = useContext(SiteInfoContext);

    if (ctx === null) {
        throw new Error("useSiteInfo must be used within a SiteInfoProvider");
    }

    return ctx;
}

export function useSiteInfo(): SiteInfo {
    const { siteInfo } = useSiteInfoState();

    if (siteInfo === null) {
        throw new Error("useSiteInfo was called before the site info loaded");
    }

    return siteInfo;
}
