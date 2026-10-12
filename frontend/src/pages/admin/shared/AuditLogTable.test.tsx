import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeAuditLogEntry } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { formatExactDateTime } from "../../../utils/time";
import { AuditLogTable } from "./AuditLogTable";

describe("AuditLogTable", () => {
    it("lists each entry with its action, actor and details", () => {
        // given
        const entry = makeAuditLogEntry({
            action: "lock_user",
            actor_name: "Sam",
            details: { reason: "chargeback" },
        });

        // when
        renderWithProviders(<AuditLogTable caption="Audit log" entries={[entry]} showSubject linkSubjects />);

        // then
        const row = screen.getAllByRole("row")[1];
        expect(within(row).getByText("User locked")).toBeInTheDocument();
        expect(within(row).getByText("Sam")).toBeInTheDocument();
        expect(within(row).getByText("Reason")).toBeInTheDocument();
        expect(within(row).getByText("chargeback")).toBeInTheDocument();
    });

    it("names the provider of each sign-in method an entry is about", () => {
        // given
        const entries = [
            makeAuditLogEntry({
                id: 1,
                action: "identity_linked",
                target_type: "identity",
                target_id: "google",
                details: { provider: "google", provider_name: "Google", method: "account_settings" },
            }),
            makeAuditLogEntry({
                id: 2,
                action: "identity_unlinked",
                target_type: "identity",
                target_id: "example",
                details: { provider: "example", provider_name: "Example ID" },
            }),
        ];

        // when
        renderWithProviders(<AuditLogTable caption="Audit log" entries={entries} showSubject linkSubjects />);

        // then
        const rows = screen.getAllByRole("row");
        expect(within(rows[1]).getByText("Sign-in method linked")).toBeInTheDocument();
        expect(within(rows[1]).getByText("Google")).toBeInTheDocument();
        expect(within(rows[1]).getByText("Provider ID")).toBeInTheDocument();
        expect(within(rows[1]).getByText("google")).toBeInTheDocument();
        expect(within(rows[2]).getByText("Sign-in method removed")).toBeInTheDocument();
        expect(within(rows[2]).getByText("Example ID")).toBeInTheDocument();
        expect(within(rows[2]).queryByText("Provider name")).not.toBeInTheDocument();
    });

    it("shows when each entry happened", () => {
        // given
        const entry = makeAuditLogEntry({ created_at: "2026-09-20T12:00:00Z" });

        // when
        renderWithProviders(<AuditLogTable caption="Audit log" entries={[entry]} showSubject linkSubjects />);

        // then
        expect(screen.getByText(formatExactDateTime("2026-09-20T12:00:00Z"))).toHaveAttribute(
            "datetime",
            "2026-09-20T12:00:00Z",
        );
    });

    it("links the user an entry is about when the viewer may open user records", () => {
        // given
        const entry = makeAuditLogEntry({ subject_id: "user-7", subject_name: "Robin" });

        // when
        renderWithProviders(<AuditLogTable caption="Audit log" entries={[entry]} showSubject linkSubjects />);

        // then
        expect(screen.getByRole("link", { name: "Robin" })).toHaveAttribute("href", "/admin/users/user-7");
    });

    it("names the user without a link when the viewer may not open user records", () => {
        // given
        const entry = makeAuditLogEntry({ subject_id: "user-7", subject_name: "Robin" });

        // when
        renderWithProviders(<AuditLogTable caption="Audit log" entries={[entry]} showSubject linkSubjects={false} />);

        // then
        expect(screen.getByText("Robin")).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Robin" })).not.toBeInTheDocument();
    });

    it("says the account is gone when the user it was about has been deleted", () => {
        // given
        const entry = makeAuditLogEntry({ subject_id: "user-7", subject_name: undefined });

        // when
        renderWithProviders(<AuditLogTable caption="Audit log" entries={[entry]} showSubject linkSubjects />);

        // then
        const row = screen.getAllByRole("row")[1];
        expect(within(row).getByText("Deleted account")).toBeInTheDocument();
        expect(within(row).queryByRole("link")).not.toBeInTheDocument();
    });

    it("leaves the user column out when every entry is about the same user", () => {
        // given
        const entry = makeAuditLogEntry();

        // when
        renderWithProviders(
            <AuditLogTable caption="Audit log" entries={[entry]} showSubject={false} linkSubjects={false} />,
        );

        // then
        expect(screen.queryByRole("columnheader", { name: "User" })).not.toBeInTheDocument();
        expect(screen.getByRole("columnheader", { name: "By" })).toBeInTheDocument();
    });
});
