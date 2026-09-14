import React, { useState } from "react";
import { Sun, Moon } from "react-feather";
import { toggleThemeWithReveal } from "./themeTransition";

// Standalone theme toggle for pages outside the admin shell (no AppNavbar there to host
// one). Self-contained - reads/writes the same localStorage key and <html data-theme>
// attribute AdminLayout uses, so flipping it here stays in sync everywhere else.
//
// `fixed` (default true) pins it to the viewport corner, floating above scroll - the
// original behavior, kept as the default for every existing caller. Pass `fixed={false}`
// to render it as a normal inline element instead, so it scrolls with the page (e.g. when
// placed inside a page's own toolbar row rather than floating over it).
const ThemeToggleButton = ({ fixed = true }) => {
    const [theme, setTheme] = useState(() => document.documentElement.getAttribute("data-theme") || "light");

    const toggle = (event) => {
        toggleThemeWithReveal(event, () => {
            const next = theme === "dark" ? "light" : "dark";
            setTheme(next);
            window.localStorage.setItem("theme", next);
            document.documentElement.setAttribute("data-theme", next);
        });
    };

    return (
        <button
            type="button"
            onClick={toggle}
            aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
            style={{
                ...(fixed ? { position: "fixed", top: 16, right: 16, zIndex: 1000 } : { position: "static" }),
                width: 36,
                height: 36,
                borderRadius: "10px",
                border: "1px solid var(--border-default)",
                background: "var(--bg-raised)",
                color: "var(--icon-default)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                cursor: "pointer",
            }}
        >
            {theme === "dark" ? (
                <Sun size={16} color="#f5c518" />
            ) : (
                <Moon size={16} color="var(--text-primary)" fill="var(--text-primary)" />
            )}
        </button>
    );
};

export default ThemeToggleButton;
