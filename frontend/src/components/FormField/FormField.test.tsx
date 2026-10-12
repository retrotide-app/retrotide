import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Select } from "../Select/Select";
import { TextArea } from "../TextArea/TextArea";
import { FormField } from "./FormField";

describe("FormField", () => {
    it("labels the control it wraps", () => {
        // given
        const ui = <FormField label="Reason">{control => <TextArea {...control} />}</FormField>;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("Reason")).toBeInstanceOf(HTMLTextAreaElement);
    });

    it("describes the control with its hint", () => {
        // given
        const ui = (
            <FormField label="Role" hint="Support staff can be edited from the permissions page.">
                {control => (
                    <Select {...control}>
                        <option value="support">Support</option>
                    </Select>
                )}
            </FormField>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("Role")).toHaveAccessibleDescription(
            "Support staff can be edited from the permissions page.",
        );
        expect(screen.getByLabelText("Role")).not.toHaveAttribute("aria-invalid");
    });

    it("ties the error to the control and marks it invalid", () => {
        // given
        const ui = (
            <FormField label="Reason" error="Give a reason">
                {control => <TextArea {...control} />}
            </FormField>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("Reason")).toHaveAccessibleDescription("Give a reason");
        expect(screen.getByLabelText("Reason")).toHaveAttribute("aria-invalid", "true");
    });

    it("reads the hint and then the error when there are both", () => {
        // given
        const ui = (
            <FormField label="Reason" hint="The user sees this." error="Give a reason">
                {control => <TextArea {...control} />}
            </FormField>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("Reason")).toHaveAccessibleDescription("The user sees this. Give a reason");
    });

    it("describes nothing when there is no hint and no error", () => {
        // given
        const ui = <FormField label="Reason">{control => <TextArea {...control} />}</FormField>;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("Reason")).not.toHaveAttribute("aria-describedby");
        expect(screen.getByLabelText("Reason")).not.toHaveAttribute("aria-invalid");
    });

    it("treats an empty error as no error", () => {
        // given
        const ui = (
            <FormField label="Reason" error="">
                {control => <TextArea {...control} />}
            </FormField>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("Reason")).not.toHaveAttribute("aria-invalid");
    });

    it("still labels and describes the control when laid out as a row", () => {
        // given
        const ui = (
            <FormField label="Reason" hint="The user sees this." error="Give a reason" layout="row">
                {control => <TextArea {...control} />}
            </FormField>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("Reason")).toHaveAccessibleDescription("The user sees this. Give a reason");
        expect(screen.getByLabelText("Reason")).toHaveAttribute("aria-invalid", "true");
    });

    it("gives each field its own id so two fields never share a label", () => {
        // given
        const ui = (
            <>
                <FormField label="First">{control => <TextArea {...control} />}</FormField>
                <FormField label="Second">{control => <TextArea {...control} />}</FormField>
            </>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByLabelText("First").id).not.toBe(screen.getByLabelText("Second").id);
    });
});
