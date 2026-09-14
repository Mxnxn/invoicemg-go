import React from "react";
import { AlertCircle } from "react-feather";

// Says what is still missing, beside a disabled submit button.
//
// The Create* modals disable submit until every required field is filled, which is what was
// asked for - but it means a "reveal all the errors on submit" pass can never run, because
// the button cannot be clicked. Without something like this the user is left with a dead
// button and no explanation, which is the exact complaint that started this work.
//
// `missing` is a list of human labels, e.g. ["Client", "Job Number"].
const MissingFieldsHint = ({ missing = [] }) => {
	if (missing.length === 0) return null;

	return (
		<span className="missing-hint" role="status">
			<AlertCircle size={14} />
			<span>
				Still needed: <strong>{missing.join(", ")}</strong>
			</span>
		</span>
	);
};

export default MissingFieldsHint;
