import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeAdminUserDetail } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { formatExactDateTime } from "../../../utils/time";
import { UserDetails } from "./UserDetails";

function detail(term: string): HTMLElement {
    const label = screen.getByText(term, { selector: "dt" });
    const item = label.parentElement;

    if (item === null) {
        throw new Error(`no detail for ${term}`);
    }

    return item;
}

describe("UserDetails", () => {
    it("shows who the user is and how their account stands", () => {
        // given
        const user = makeAdminUserDetail({
            email: "robin@example.test",
            email_verified: true,
            role: "support",
            has_password: false,
            created_at: "2026-03-01T08:00:00Z",
        });

        // when
        renderWithProviders(<UserDetails user={user} />);

        // then
        expect(detail("Email")).toHaveTextContent("robin@example.test");
        expect(detail("Email")).toHaveTextContent("Verified");
        expect(detail("Role")).toHaveTextContent("Support");
        expect(detail("Status")).toHaveTextContent("Active");
        expect(detail("Password")).toHaveTextContent("Not set");
        expect(detail("Joined")).toHaveTextContent(formatExactDateTime("2026-03-01T08:00:00Z"));
    });

    it("says when the address is unverified and the user holds no role", () => {
        // given
        const user = makeAdminUserDetail({ email_verified: false, role: undefined });

        // when
        renderWithProviders(<UserDetails user={user} />);

        // then
        expect(detail("Email")).toHaveTextContent("Unverified");
        expect(detail("Role")).toHaveTextContent("No role");
    });

    it("shows why and when a user was banned and locked", () => {
        // given
        const user = makeAdminUserDetail({
            banned: true,
            banned_at: "2026-09-01T10:00:00Z",
            ban_reason: "spam",
            locked: true,
            locked_at: "2026-09-02T10:00:00Z",
            lock_reason: "chargeback",
        });

        // when
        renderWithProviders(<UserDetails user={user} />);

        // then
        expect(detail("Status")).toHaveTextContent("Banned");
        expect(detail("Status")).toHaveTextContent("Locked");
        expect(detail("Status")).not.toHaveTextContent("Active");
        expect(detail("Banned on")).toHaveTextContent(formatExactDateTime("2026-09-01T10:00:00Z"));
        expect(detail("Ban reason")).toHaveTextContent("spam");
        expect(detail("Locked on")).toHaveTextContent(formatExactDateTime("2026-09-02T10:00:00Z"));
        expect(detail("Lock reason")).toHaveTextContent("chargeback");
    });

    it("shows where the user was last seen", () => {
        // given
        const user = makeAdminUserDetail({ last_seen_ip: "203.0.113.9", last_seen_ip_at: "2026-09-20T12:00:00Z" });

        // when
        renderWithProviders(<UserDetails user={user} />);

        // then
        expect(detail("Last seen from")).toHaveTextContent(
            `203.0.113.9 on ${formatExactDateTime("2026-09-20T12:00:00Z")}`,
        );
    });

    it("leaves out what the record does not carry", () => {
        // given
        const user = makeAdminUserDetail();

        // when
        renderWithProviders(<UserDetails user={user} />);

        // then
        expect(screen.queryByText("Ban reason")).not.toBeInTheDocument();
        expect(screen.queryByText("Lock reason")).not.toBeInTheDocument();
        expect(screen.queryByText("Last seen from")).not.toBeInTheDocument();
    });
});
