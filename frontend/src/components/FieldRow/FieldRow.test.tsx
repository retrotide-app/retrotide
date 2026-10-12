import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { TextField } from "../FormField/TextField";
import { FieldRow } from "./FieldRow";

describe("FieldRow", () => {
    it("keeps the fields it is given, in order", () => {
        // when
        renderWithProviders(
            <FieldRow>
                <TextField label="Person" />
                <TextField label="Role" />
            </FieldRow>,
        );

        // then
        const fields = screen.getAllByRole("textbox");
        expect(fields).toEqual([screen.getByLabelText("Person"), screen.getByLabelText("Role")]);
    });
});
