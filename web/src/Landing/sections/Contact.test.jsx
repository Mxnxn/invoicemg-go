import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Contact } from "./Contact";
import * as enquiry from "../lib/enquiry";

afterEach(() => vi.restoreAllMocks());

function fill({ name = "Ramesh Patel", email = "ramesh@example.com", phone = "9999999999" } = {}) {
    fireEvent.change(screen.getByLabelText(/Your name/), { target: { value: name } });
    fireEvent.change(screen.getByLabelText(/^Email/), { target: { value: email } });
    fireEvent.change(screen.getByLabelText(/^Phone/), { target: { value: phone } });
}

describe("Contact form", () => {
    it("is anchored so the CTA button can reach it", () => {
        const { container } = render(<Contact />);
        expect(container.querySelector("#contact")).not.toBeNull();
    });

    it("shows the country code without making the visitor type it", () => {
        render(<Contact />);
        expect(screen.getByText("+91")).toBeDefined();
    });

    it("names the field that is missing rather than a generic error", () => {
        render(<Contact />);
        fireEvent.click(screen.getByRole("button", { name: /Request a demo/ }));
        expect(screen.getByText("Name is required.")).toBeDefined();
    });

    it("rejects an email that is not one", () => {
        render(<Contact />);
        fill({ email: "nope" });
        fireEvent.click(screen.getByRole("button", { name: /Request a demo/ }));
        expect(screen.getByText("Enter a valid email address.")).toBeDefined();
    });

    it("submits the phone in E.164, not as typed", async () => {
        const submit = vi.spyOn(enquiry, "submitEnquiry").mockResolvedValue({ code: 200, status: true });
        render(<Contact />);
        fill({ phone: "99999 99999" });
        fireEvent.click(screen.getByRole("button", { name: /Request a demo/ }));

        await waitFor(() => expect(submit).toHaveBeenCalled());
        expect(submit.mock.calls[0][0].phone).toBe("+919999999999");
    });

    it("thanks the visitor once the enquiry is recorded", async () => {
        vi.spyOn(enquiry, "submitEnquiry").mockResolvedValue({ code: 200, status: true });
        render(<Contact />);
        fill();
        fireEvent.click(screen.getByRole("button", { name: /Request a demo/ }));

        await waitFor(() => expect(screen.getByText(/Thanks, Ramesh/)).toBeDefined());
    });

    it("keeps the form up when the request is refused", async () => {
        vi.spyOn(enquiry, "submitEnquiry").mockResolvedValue({ code: 429, status: false });
        render(<Contact />);
        fill();
        fireEvent.click(screen.getByRole("button", { name: /Request a demo/ }));

        // The global interceptor raises the toast; the form must not pretend it succeeded.
        await waitFor(() => expect(screen.getByRole("button", { name: /Request a demo/ })).toBeDefined());
        expect(screen.queryByText(/Thanks, Ramesh/)).toBeNull();
    });

    it("carries a honeypot that is hidden from people", () => {
        render(<Contact />);
        const pot = document.getElementById("enquiry-website");
        expect(pot).not.toBeNull();
        expect(pot.getAttribute("autocomplete")).toBe("off");
        expect(pot.getAttribute("tabindex")).toBe("-1");
    });
});

describe("Contact phone field", () => {
    it("keeps the country-code group the same height as a plain field", () => {
        render(<Contact />);
        const phoneWrap = document.getElementById("enquiry-phone").parentElement;
        // Padding belongs to the input, not the wrapper - having it on both made this
        // group taller than every other field on the form.
        expect(phoneWrap.className).not.toMatch(/tw:py-/);
        expect(phoneWrap.className).toMatch(/tw:items-stretch/);
    });

    it("shows a focus ring, since the inner input suppresses its own", () => {
        render(<Contact />);
        const phoneWrap = document.getElementById("enquiry-phone").parentElement;
        expect(phoneWrap.className).toMatch(/focus-within:outline-ring/);
    });
});

describe("after the demo request is sent", () => {
    // Stubs the request the way every other successful-submit test here does, and reuses the
    // file's own `fill` for the fields. Without the stub the real submitEnquiry runs and the
    // confirmation panel never appears.
    const sendIt = async () => {
        vi.spyOn(enquiry, "submitEnquiry").mockResolvedValue({ code: 200, status: true });
        render(<Contact />);
        fill();
        fireEvent.click(screen.getByRole("button", { name: /Request a demo/ }));
        await waitFor(() => expect(screen.getByText(/Thanks/)).toBeDefined());
    };

    // The whole point of asking for a demo is to see one. Leaving them on a "we'll be in
    // touch" note makes them wait for an email to do the thing they just asked to do.
    it("offers the demo straight away", async () => {
        await sendIt();
        const link = screen.getByText("Open the demo").closest("a");
        expect(link.getAttribute("href")).toMatch(/\/admin$/);
    });

    // A login screen with no way in is a dead end.
    it("gives them the credentials to get in", async () => {
        await sendIt();
        expect(screen.getByText("demo@invoicemg.in")).toBeDefined();
        expect(screen.getByText("demo1234")).toBeDefined();
    });

    // The WhatsApp handoff was there first and must survive the addition.
    //
    // whatsappHandoffUrl is stubbed because it returns "" unless VITE_ENQUIRY_WHATSAPP is
    // set, and the button is only rendered when it returns something. A local .env supplies
    // that variable and CI does not - so without the stub this passed on a laptop and failed
    // in the pipeline, which is worse than failing everywhere.
    it("keeps the WhatsApp option", async () => {
        vi.spyOn(enquiry, "whatsappHandoffUrl").mockReturnValue("https://wa.me/919000000000?text=hi");
        await sendIt();
        expect(screen.getByText(/Continue on WhatsApp/)).toBeDefined();
    });

    // And the other half of that behaviour, which nothing covered: with no business number
    // configured there is no handoff to offer, and the panel must simply not show one rather
    // than render a link to nowhere.
    it("omits the WhatsApp option when no business number is configured", async () => {
        vi.spyOn(enquiry, "whatsappHandoffUrl").mockReturnValue("");
        await sendIt();
        expect(screen.queryByText(/Continue on WhatsApp/)).toBeNull();
        // The demo hand-off does not depend on that number, so it must still be there.
        expect(screen.getByText("Open the demo")).toBeDefined();
    });
});
