import { Link } from "react-router-dom";
import { ReceiptTextIcon } from "lucide-react";

import { siteConfig } from "./lib/site-config";

export function SiteFooter() {
    return (
        <footer className="tw:border-t tw:border-border tw:py-12">
            <div className="tw:mx-auto tw:flex tw:max-w-6xl tw:flex-col tw:gap-6 tw:px-4 tw:lg:flex-row tw:lg:items-center tw:lg:justify-between tw:lg:px-8">
                <div>
                    <span className="tw:flex tw:items-center tw:gap-2 tw:font-semibold">
                        <ReceiptTextIcon className="tw:size-5 tw:text-primary" />
                        {siteConfig.name}
                    </span>
                    <p className="tw:mt-2 tw:max-w-sm tw:text-sm tw:text-muted-foreground">{siteConfig.tagline}</p>
                </div>
                <nav className="tw:flex tw:flex-wrap tw:gap-6 tw:text-sm tw:text-muted-foreground">
                    <a href="#features" className="tw:transition-colors tw:hover:text-foreground">
                        Features
                    </a>
                    <a href="#workflow" className="tw:transition-colors tw:hover:text-foreground">
                        Workflow
                    </a>
                    <a href="#faq" className="tw:transition-colors tw:hover:text-foreground">
                        FAQ
                    </a>
                    <Link to={siteConfig.adminPath} className="tw:transition-colors tw:hover:text-foreground">
                        Dashboard
                    </Link>
                </nav>
                <p className="tw:text-sm tw:text-muted-foreground">
                    © {new Date().getFullYear()} {siteConfig.name}
                </p>
            </div>
        </footer>
    );
}
