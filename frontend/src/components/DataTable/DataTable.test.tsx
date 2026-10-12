import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { DataTable } from "./DataTable";

describe("DataTable", () => {
    it("names the table after its caption", () => {
        // given
        const caption = "Users";

        // when
        renderWithProviders(
            <DataTable caption={caption}>
                <thead>
                    <tr>
                        <th scope="col">Name</th>
                    </tr>
                </thead>
                <tbody>
                    <tr>
                        <td>Robin</td>
                    </tr>
                </tbody>
            </DataTable>,
        );

        // then
        const table = screen.getByRole("table", { name: caption });
        expect(within(table).getByRole("columnheader", { name: "Name" })).toBeInTheDocument();
        expect(within(table).getByRole("cell", { name: "Robin" })).toBeInTheDocument();
    });

    it("lets keyboard users scroll a wide table", () => {
        // given
        const caption = "Audit log";

        // when
        renderWithProviders(
            <DataTable caption={caption}>
                <tbody>
                    <tr>
                        <td>entry</td>
                    </tr>
                </tbody>
            </DataTable>,
        );

        // then
        expect(screen.getByRole("region", { name: caption })).toHaveAttribute("tabindex", "0");
    });
});
