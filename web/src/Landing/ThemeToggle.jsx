import { useState } from "react";
import { MoonIcon, SunIcon } from "lucide-react";

import { Button } from "./ui/Button";

function currentTheme() {
    if (typeof document === "undefined") return "light";
    return document.documentElement.getAttribute("data-theme") || "light";
}

/**
 * Mirrors AdminLayout's theme switch: writes localStorage.theme and the
 * html[data-theme] attribute that src/index.js reads before first paint, so a
 * visitor's choice on the landing page carries into the admin panel.
 */
export function ThemeToggle() {
    const [theme, setTheme] = useState(currentTheme);

    function toggle() {
        const next = theme === "dark" ? "light" : "dark";
        document.documentElement.setAttribute("data-theme", next);
        window.localStorage.setItem("theme", next);
        setTheme(next);
    }

    return (
        <Button variant="ghost" size="icon" aria-label="Toggle theme" onClick={toggle}>
            {theme === "dark" ? <SunIcon /> : <MoonIcon />}
        </Button>
    );
}
