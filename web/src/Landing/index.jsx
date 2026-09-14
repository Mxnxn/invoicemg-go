import { useRef } from "react";

import "./landing.css";

import { SiteHeader } from "./SiteHeader";
import { SiteFooter } from "./SiteFooter";
import { ScrollProgress } from "./velora/scroll-progress";
import { useSmoothAnchors } from "./useSmoothAnchors";
import { Hero } from "./sections/Hero";
import { Stats } from "./sections/Stats";
import { Features } from "./sections/Features";
import { Workflow } from "./sections/Workflow";
import { Activity } from "./sections/Activity";
import { Differentiators } from "./sections/Differentiators";
import { Faq } from "./sections/Faq";
import { Cta } from "./sections/Cta";
import { Contact } from "./sections/Contact";

/**
 * Static marketing page served at /. Lazy-loaded from App.js so neither its
 * JavaScript nor its Tailwind stylesheet reaches an admin route.
 */
export default function LandingPage() {
    const rootRef = useRef(null);
    // In-page links glide to their section instead of jumping. Scoped to this root rather
    // than set on <html>, which the admin app shares - see useSmoothAnchors.
    useSmoothAnchors(rootRef);

    return (
        <div className="velora-root" ref={rootRef}>
            <ScrollProgress />
            <SiteHeader />
            <main>
                <Hero />
                <Stats />
                <Features />
                <Workflow />
                <Activity />
                <Differentiators />
                <Faq />
                <Cta />
                <Contact />
            </main>
            <SiteFooter />
        </div>
    );
}
