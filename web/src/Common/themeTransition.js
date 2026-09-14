import { flushSync } from "react-dom";

// Circular "reveal" animation for theme toggles, expanding from the click position via
// the View Transition API. No polyfill exists for browsers that lack it (Firefox, Safari
// < 18), so those just fall back to an instant swap - same as before this existed.
export function toggleThemeWithReveal(event, applyTheme) {
    const supportsViewTransitions = typeof document.startViewTransition === "function";
    const prefersReducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!supportsViewTransitions || prefersReducedMotion) {
        applyTheme();
        return;
    }

    const x = event?.clientX ?? window.innerWidth / 2;
    const y = event?.clientY ?? window.innerHeight / 2;
    const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y));

    const transition = document.startViewTransition(() => flushSync(applyTheme));

    transition.ready.then(() => {
        document.documentElement.animate(
            {
                clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`],
            },
            {
                duration: 450,
                easing: "ease-in-out",
                pseudoElement: "::view-transition-new(root)",
            }
        );
    });
}
