import acceptableUse from "../../content/legal/acceptable-use.md?raw";
import cookies from "../../content/legal/cookies.md?raw";
import privacy from "../../content/legal/privacy.md?raw";
import security from "../../content/legal/security.md?raw";
import subProcessors from "../../content/legal/sub-processors.md?raw";
import terms from "../../content/legal/terms.md?raw";

export interface LegalDocument {
    title: string;
    source: string;
}

export const LEGAL_DOCUMENTS = new Map<string, LegalDocument>([
    ["privacy", { title: "Privacy notice", source: privacy }],
    ["terms", { title: "Terms of service", source: terms }],
    ["acceptable-use", { title: "Acceptable use policy", source: acceptableUse }],
    ["cookies", { title: "Cookie statement", source: cookies }],
    ["sub-processors", { title: "Sub-processors", source: subProcessors }],
    ["security", { title: "Security", source: security }],
]);
