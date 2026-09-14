// How a phone number typed by a human becomes the ten digits we store.
//
// Every form used the rule /^\d{10}$/ against the raw input, which rejected the exact
// format the fields advertised in their own placeholder ("+91 98765 43210"). Because a
// failed rule also disabled the submit button, and an error only renders once the field
// has been blurred, typing the suggested format produced a dead button and no explanation.
//
// Accept what people actually type - spaces, hyphens, brackets, a +91 country code, a
// leading 0 - and normalise it to the ten digits before validating or saving.

export const normalizePhone = (value) => {
    const digits = String(value ?? "").replace(/\D/g, "");
    // 91 is only a country code when it prefixes a full number; a real 10-digit number may
    // legitimately begin with 91 (9198765432) and must survive untouched.
    if (digits.length === 12 && digits.startsWith("91")) return digits.slice(2);
    if (digits.length === 11 && digits.startsWith("0")) return digits.slice(1);
    return digits;
};

export const isValidPhone = (value) => /^\d{10}$/.test(normalizePhone(value));

// Drop-in for useRequiredFields' `validate`.
export const phoneRule = (value) =>
    isValidPhone(value) ? null : "Phone must be a 10-digit number.";
