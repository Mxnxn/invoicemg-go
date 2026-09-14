import React, { useState } from "react";
import { Eye, EyeOff } from "react-feather";
import "./passwordInput.css";

// A password field with a reveal toggle. Wraps whatever input markup the caller already
// uses rather than imposing one: pass `className` and the usual input props through, and
// the toggle is positioned over the right-hand edge.
//
// The toggle is a <button type="button"> on purpose - inside a <form> a bare <button>
// defaults to type="submit", so revealing the password would submit the form.
const PasswordInput = ({
	className = "",
	inputClassName = "",
	wrapperStyle,
	autoComplete = "new-password",
	// Named rather than a forwardRef: every call site here passes plain props, and one more
	// prop is less surprising than a component that is a ref target in only one place.
	innerRef,
	...inputProps
}) => {
	const [visible, setVisible] = useState(false);

	return (
		<span className={["password-input", className].filter(Boolean).join(" ")} style={wrapperStyle}>
			<input
				{...inputProps}
				ref={innerRef}
				type={visible ? "text" : "password"}
				className={inputClassName}
				autoComplete={autoComplete}
			/>
			<button
				type="button"
				className="password-input-toggle"
				onClick={() => setVisible((v) => !v)}
				aria-label={visible ? "Hide password" : "Show password"}
				aria-pressed={visible}
				tabIndex={-1}
			>
				{visible ? <EyeOff size={16} /> : <Eye size={16} />}
			</button>
		</span>
	);
};

export default PasswordInput;
