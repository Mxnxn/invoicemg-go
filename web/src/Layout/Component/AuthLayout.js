import React, { useLayoutEffect, useState } from "react";
import AuthForm from "../../Views/Auth/components/AuthForm";
import ThemeToggleButton from "../../Common/ThemeToggleButton";

const AuthLayout = () => {
    // Unauthenticated visitors have never toggled a theme yet, so this page defaults to
    // dark rather than inheriting the app-wide "light" default AdminLayout uses.
    const [theme, setTheme] = useState(() => window.localStorage.getItem("theme") || "dark");

    useLayoutEffect(() => {
        window.localStorage.setItem("theme", theme);
        document.documentElement.setAttribute("data-theme", theme);
    }, [theme]);

    return (
        <div className="auth-page" data-shell data-theme={theme}>
            <span className="auth-brand">InvoiceMG</span>
            <ThemeToggleButton fixed />
            <AuthForm />
        </div>
    );
};

export default AuthLayout;
