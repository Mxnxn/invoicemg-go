import { useCallback, useMemo, useState } from "react";

// Shared required-field handling for forms.
//
// Three things every form in this app was doing differently, or not at all:
//   1. marking which inputs are mandatory,
//   2. saying WHICH one is missing rather than one generic error,
//   3. keeping submit disabled until the form can actually succeed.
//
// Usage:
//   const req = useRequiredFields(form, { clientName: "Client name", clientPhone: "Phone" });
//   <input {...req.props("clientName")} />
//   <button disabled={!req.isComplete}>Save</button>
//
// `rules` maps field name -> label, or field name -> { label, validate }. A validate function
// returns an error string, or anything falsy when the value is acceptable.
export default function useRequiredFields(values, rules) {
	const [touched, setTouched] = useState({});

	const errors = useMemo(() => {
		const out = {};
		Object.keys(rules || {}).forEach((field) => {
			const rule = rules[field];
			const label = typeof rule === "string" ? rule : rule.label || field;
			const raw = values ? values[field] : undefined;
			const value = typeof raw === "string" ? raw.trim() : raw;

			if (value === undefined || value === null || value === "") {
				out[field] = `${label} is required.`;
				return;
			}
			if (typeof rule === "object" && typeof rule.validate === "function") {
				const message = rule.validate(value, values);
				if (message) out[field] = message;
			}
		});
		return out;
	}, [values, rules]);

	const isComplete = Object.keys(errors).length === 0;

	const markTouched = useCallback((field) => {
		setTouched((prev) => (prev[field] ? prev : { ...prev, [field]: true }));
	}, []);

	// Reveal every error at once - used when submit is attempted, so the user sees all the
	// gaps rather than discovering them one at a time.
	const showAll = useCallback(() => {
		const all = {};
		Object.keys(rules || {}).forEach((f) => {
			all[f] = true;
		});
		setTouched(all);
	}, [rules]);

	const errorFor = useCallback((field) => (touched[field] ? errors[field] : undefined), [touched, errors]);

	// Spread onto an input to get the invalid styling and blur tracking for free.
	const props = useCallback(
		(field) => ({
			onBlur: () => markTouched(field),
			"aria-invalid": Boolean(errorFor(field)),
			className: errorFor(field) ? "is-required-missing" : undefined,
		}),
		[markTouched, errorFor]
	);

	return { errors, errorFor, isComplete, showAll, markTouched, props, touched };
}
