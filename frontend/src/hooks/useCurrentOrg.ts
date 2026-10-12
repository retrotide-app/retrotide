import { useContext } from "react";
import { OrgContext, type OrgContextValue } from "../context/orgContextValue";

export function useCurrentOrg(): OrgContextValue {
    const ctx = useContext(OrgContext);

    if (ctx === null) {
        throw new Error("useCurrentOrg must be used within an OrgProvider");
    }

    return ctx;
}
