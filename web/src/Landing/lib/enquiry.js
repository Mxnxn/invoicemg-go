import axios from "axios";

// The country code the form assumes. Typed numbers are local; a pasted international
// number keeps its own code.
export const DEFAULT_DIAL_CODE = "+91";

// Where the wa.me handoff goes. Digits only, no plus - that is the format wa.me takes.
const BUSINESS_WHATSAPP = String(import.meta.env.VITE_ENQUIRY_WHATSAPP || "").replace(/\D/g, "");

/**
 * Normalise what someone typed into E.164.
 * "99999 99999" -> "+919999999999", "+44 20 7946 0958" -> "+442079460958".
 * Returns "" when there is nothing usable, so the caller can show its own error.
 */
export function normalisePhone(input, dialCode = DEFAULT_DIAL_CODE) {
    const raw = String(input || "").trim();
    if (!raw) return "";
    if (raw.startsWith("+")) {
        const digits = raw.slice(1).replace(/\D/g, "");
        return digits ? `+${digits}` : "";
    }
    const digits = raw.replace(/\D/g, "");
    if (!digits) return "";
    // A number pasted with the country code but no plus ("919999999999").
    const code = dialCode.replace(/\D/g, "");
    if (digits.startsWith(code) && digits.length > code.length) return `+${digits}`;
    return `${dialCode}${digits}`;
}

/**
 * The first name, capitalised for greeting them back. The server title-cases what it
 * stores (Helpers/TextCase.js), so echoing the raw input would greet "sunil" while the
 * record says "Sunil Verma".
 */
export function greetingName(fullName) {
    const first = String(fullName || "").trim().split(/\s+/)[0] || "";
    return first ? first.charAt(0).toUpperCase() + first.slice(1) : "";
}

/** The message the visitor's own WhatsApp opens with. */
export function enquiryMessage({ name, email, phone, companyName, note }) {
    const lines = [
        "Hello InvoiceMG, I would like a demo.",
        "",
        `Name: ${name}`,
        companyName ? `Company: ${companyName}` : "",
        `Phone: ${phone}`,
        `Email: ${email}`,
        note ? `\nWhat I need: ${note}` : "",
    ].filter(Boolean);
    return lines.join("\n");
}

/**
 * The wa.me handoff - option one of the two paths an enquiry can take. It needs no
 * template approval because the visitor sends it from their own WhatsApp; it is their
 * message, not ours. Returns "" when no business number is configured, so the caller can
 * fall back to the stored enquiry alone.
 */
export function whatsappHandoffUrl(values) {
    if (!BUSINESS_WHATSAPP) return "";
    return `https://wa.me/${BUSINESS_WHATSAPP}?text=${encodeURIComponent(enquiryMessage(values))}`;
}

/**
 * Record the enquiry. The token is a speed bump against drive-by bots, not a secret - it
 * ships in this bundle and anyone can read it. The route's per-IP rate limit is the guard
 * that actually bounds abuse.
 */
export async function submitEnquiry(values) {
    const body = new FormData();
    body.set("name", values.name);
    body.set("email", values.email);
    body.set("phone", values.phone);
    body.set("companyName", values.companyName || "");
    body.set("note", values.note || "");
    body.set("source", "landing");
    // The honeypot. Always empty from a real person - the input is hidden from view and
    // from assistive tech, and carries autocomplete="off" so a browser will not fill it.
    body.set("website", values.website || "");

    const res = await axios.post(`${import.meta.env.VITE_API_URL}/enquiry`, body, {
        headers: { "X-ENQUIRY-TOKEN": import.meta.env.VITE_ENQUIRY_TOKEN || "" },
    });
    return res.data;
}
