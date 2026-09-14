import { useEffect } from "react";

// Makes the landing page's in-page links glide to their section instead of teleporting.
//
// Every "Request a demo" and "Contact us" is an <a href="#contact">, which the browser jumps
// to instantly - the reader arrives with no sense of having travelled, and on a long page it
// reads as though the whole page was replaced.
//
// WHY A LISTENER AND NOT `scroll-behavior: smooth`
//
// That property has to sit on the SCROLLING element, which is <html> - shared with the whole
// admin app. landing.css is only in the document on the landing route, but the chunk stays
// loaded once a visitor navigates on to /admin in the same SPA session, so the rule would
// outlive the page it was written for. A listener scoped to the landing root cannot.
//
// Delegated from one root node rather than bound per link, so sections rendered later (or
// conditionally) are covered without registering anything of their own.
export function useSmoothAnchors(rootRef) {
    useEffect(() => {
        const root = rootRef.current;
        if (!root) return undefined;

        const onClick = (event) => {
            // Let the browser handle anything it would normally treat specially: a new tab, a
            // download, a modified click. Hijacking those is how "open in new tab" breaks.
            if (event.defaultPrevented || event.button !== 0) return;
            if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;

            const link = event.target.closest('a[href^="#"]');
            if (!link || !root.contains(link)) return;

            const id = link.getAttribute("href").slice(1);
            if (!id) return;
            const target = document.getElementById(id);
            // A link to a section that is not on the page should still do whatever it did
            // before rather than silently doing nothing.
            if (!target) return;

            event.preventDefault();

            // Honoured explicitly: "smooth" ignores prefers-reduced-motion in every engine, so
            // a reader who asked for less movement would get a long glide anyway.
            const reduced = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches;
            target.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "start" });

            // Keep the URL honest without a second jump - pushing the hash normally would make
            // the browser snap to the same element the animation is already travelling toward.
            if (window.history?.replaceState) {
                window.history.replaceState(null, "", `#${id}`);
            }

            // The scroll moves the viewport, not focus. Without this a keyboard or screen
            // reader user stays where they were, and the next Tab continues from the old
            // place - the page appears to move for everyone except them.
            target.setAttribute("tabindex", "-1");
            target.focus({ preventScroll: true });
        };

        root.addEventListener("click", onClick);
        return () => root.removeEventListener("click", onClick);
    }, [rootRef]);
}

export default useSmoothAnchors;
