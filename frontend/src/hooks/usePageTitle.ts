import { useEffect } from "react";
import { useSiteInfo } from "./useSiteInfo";

const FIRST_STRONG_ISOLATE = String.fromCharCode(0x2068);
const POP_DIRECTIONAL_ISOLATE = String.fromCharCode(0x2069);

export function usePageTitle(title?: string): void {
    const { site_name } = useSiteInfo();

    useEffect(() => {
        if (!title) {
            document.title = site_name;
            return;
        }

        document.title = `${FIRST_STRONG_ISOLATE}${title}${POP_DIRECTIONAL_ISOLATE} | ${site_name}`;
    }, [title, site_name]);
}
