import { Link } from "react-router-dom";
import { ReceiptTextIcon } from "lucide-react";

import { Button } from "./ui/Button";
import { ThemeToggle } from "./ThemeToggle";
import { siteConfig } from "./lib/site-config";

const NAV = [
    { href: "#features", label: "Features" },
    { href: "#workflow", label: "Workflow" },
    { href: "#faq", label: "FAQ" },
    { href: "#contact", label: "Contact" },
];

/** Reading uid directly mirrors ProtectiveRoute - auth state lives in localStorage, not Redux. */
function isSignedIn() {
    return Boolean(window.localStorage.getItem("uid"));
}

export function SiteHeader() {
    const signedIn = isSignedIn();

    return (
        <header className="tw:fixed tw:inset-x-0 tw:top-0 tw:z-50 tw:border-b tw:border-border tw:bg-background/70 tw:backdrop-blur-xl">
            <div className="tw:mx-auto tw:flex tw:h-16 tw:max-w-6xl tw:items-center tw:justify-between tw:px-4 tw:lg:px-8">
                <Link to="/" className="tw:flex tw:items-center tw:gap-2 tw:font-semibold">
                    <ReceiptTextIcon className="tw:size-5 tw:text-primary" />
                    {siteConfig.name}
                </Link>
                <nav className="tw:hidden tw:items-center tw:gap-6 tw:text-sm tw:text-muted-foreground tw:md:flex">
                    {NAV.map((item) => (
                        <a key={item.href} href={item.href} className="tw:transition-colors tw:hover:text-foreground">
                            {item.label}
                        </a>
                    ))}
                </nav>
                <div className="tw:flex tw:items-center tw:gap-2">
                    <ThemeToggle />
                    {/* The page's primary action: there is no price list, so the next step is
                        always a conversation. Hidden on the narrowest screens, where it would
                        crowd out the sign-in button. */}
                    <a href="#contact" className="tw:hidden tw:sm:inline-flex">
                        <Button variant="outline" size="sm">
                            Request a demo
                        </Button>
                    </a>
                    {/* Link wraps Button rather than shadcn's asChild - our Button has no Slot. */}
                    <Link to={siteConfig.adminPath}>
                        <Button size="sm">{signedIn ? "Go to dashboard" : "Sign in"}</Button>
                    </Link>
                </div>
            </div>
        </header>
    );
}
