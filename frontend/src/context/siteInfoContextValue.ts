import { createContext } from "react";
import type { SiteInfo } from "../types/app";

export interface SiteInfoContextValue {
    siteInfo: SiteInfo | null;
    loading: boolean;
    error: Error | null;
    retry: () => Promise<unknown>;
}

export const SiteInfoContext = createContext<SiteInfoContextValue | null>(null);
