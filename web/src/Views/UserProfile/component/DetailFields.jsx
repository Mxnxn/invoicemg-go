import React from "react";
import { Input } from "reactstrap";

// The label/input grid shared by the Profile and Company forms on /admin/accounts.
//
// Both describe the same kind of thing - a name, a firm, a GSTIN, bank details - and both had
// their own copy of the markup. Two copies of a form is two places to fix a label, and two
// chances for the field sizes to drift apart, which docs/STYLE.md rule 9 exists to prevent.
//
// `fields` is [key, label, placeholder]. `required` is a useRequiredFields instance, so the
// asterisk, the red border and the message all come from the same source as the submit gate.
const DetailFields = ({ idPrefix, fields, form, required, onChange, wide = ["address"] }) => (
    <div className="company-carousel-fields">
        {fields.map(([key, label, hint]) => (
            <div key={key} className={wide.includes(key) ? "is-wide" : ""}>
                <label className="form-control-label pp fs-12" htmlFor={`${idPrefix}-${key}`}>
                    {label}
                    {required.errors[key] !== undefined && <span className="required-star">*</span>}
                </label>
                <Input
                    id={`${idPrefix}-${key}`}
                    className={required.errorFor(key) ? "is-required-missing" : undefined}
                    name={key}
                    placeholder={hint}
                    value={form[key] || ""}
                    onChange={onChange}
                    onBlur={() => required.markTouched(key)}
                />
                {required.errorFor(key) && <span className="field-error">{required.errorFor(key)}</span>}
            </div>
        ))}
    </div>
);

export default DetailFields;
