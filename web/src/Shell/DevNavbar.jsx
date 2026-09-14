import React from "react";
import { Menu, Sun, Moon, LogOut } from "react-feather";
import DevTokenMenu from "./DevTokenMenu";
import { signOut } from "../Common/signOut";
import "./shell.css";

// The superadmin's top bar. Same shell chrome as AppNavbar minus everything tenant-scoped:
// no trash (that is a customer's own soft-delete bin) and no analytics source toggle.
const DevNavbar = ({ heading, theme, onToggleTheme, onOpenMobileNav }) => {
    // One shared sign-out: the local session always ends, whatever the server says.
    // See Common/signOut.js for what the two hand-rolled versions got wrong.
    const logout = () => signOut();

    return (
        <header className="shell-navbar">
            <button type="button" className="shell-hamburger" aria-label="Open navigation" onClick={onOpenMobileNav}>
                <Menu size={20} />
            </button>
            <span className="text-heading-page">{heading}</span>
            <div className="shell-navbar-spacer" />
            {/* Reachable from every dev screen, rather than only the bottom of one page. */}
            <DevTokenMenu />
            <button
                className="shell-icon-btn"
                type="button"
                aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
                onClick={onToggleTheme}
            >
                {theme === "dark" ? (
                    <Sun size={16} color="#f5c518" />
                ) : (
                    <Moon size={16} color="var(--text-primary)" fill="var(--text-primary)" />
                )}
            </button>
            <button className="shell-icon-btn" type="button" aria-label="Logout" onClick={logout}>
                <LogOut size={16} />
            </button>
        </header>
    );
};

export default DevNavbar;
