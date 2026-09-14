import React from "react";
import { Navigate, useLocation } from "react-router-dom";

import AuthLayout from "../../Layout/Component/AuthLayout";
import { hasAccess } from "../../Common/access";
import { PATH_FEATURES } from "../../Common/features";

export const ProtectiveRoute = ({ children }) => {
	const [darkMode] = React.useState(
		window.localStorage.getItem("mode")
			? window.localStorage.getItem("mode")
			: "false"
	);
	const location = useLocation();
	const uid = window.localStorage.getItem("uid");

	if (!uid) {
		return <AuthLayout />;
	}

	const role = window.localStorage.getItem("role") || "admin";
	if (role === "employee") {
		const path = location.pathname;
		const match = PATH_FEATURES.find((el) => path.startsWith(el.prefix));
		if (!match || !hasAccess(match.key)) {
			const fallback = PATH_FEATURES.find((el) => el.prefix.startsWith("/admin/") && hasAccess(el.key));
			return <Navigate to={fallback ? fallback.prefix : "/admin/lifecycle"} replace />;
		}
	}

	return React.cloneElement(children, { uid, darkModeFlag: darkMode });
};
