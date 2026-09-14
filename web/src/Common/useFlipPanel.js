import { useCallback, useEffect, useRef, useState } from "react";

/**
 * A panel that grows out of the thing that opened it, and shrinks back into it on the way out.
 *
 * Extracted because there are two of these now - the board, and a card's editor on top of it -
 * and the technique has enough detail in it that two copies would disagree within a week.
 *
 * The technique (FLIP):
 *
 *   1. The panel is laid out at its FINAL geometry by CSS. Nothing here resizes it.
 *   2. It is measured, and the difference between that and the origin rect is expressed as one
 *      transform - a translate plus a scale.
 *   3. That transform is applied, painted, and then removed with a transition.
 *
 * Transform rather than top/left/width/height, because those are layout properties: animating
 * them re-lays-out the panel's contents on every frame, so the contents reflow the whole way
 * instead of the panel moving as one piece. A transform is composited.
 *
 * The final geometry is MEASURED rather than recomputed here. It lives in the stylesheet, and
 * the same layout expressed twice in two languages is free to drift the moment either changes.
 */

const transformTo = (originRect, panelRect) => {
    if (!originRect || !panelRect || !panelRect.width || !panelRect.height) return "none";
    const sx = originRect.width / panelRect.width;
    const sy = originRect.height / panelRect.height;
    return `translate(${originRect.left - panelRect.left}px, ${originRect.top - panelRect.top}px) scale(${sx}, ${sy})`;
};

// Long enough to cover --dur-slow with room to spare. Only a fallback: transitionend is what
// normally ends the close.
const SAFETY_MS = 400;

export default function useFlipPanel({ rect, onClose }) {
    const panelRef = useRef(null);
    const closingRef = useRef(false);
    const [transform, setTransform] = useState("none");
    const [phase, setPhase] = useState("measuring");

    // Measure, place over the origin, then release - each on its own frame, because the browser
    // has to paint the panel sitting over the origin before removing the transform has anything
    // to animate from.
    useEffect(() => {
        if (!rect) return setPhase("opening");
        setTransform(transformTo(rect, panelRef.current?.getBoundingClientRect()));
        setPhase("placed");

        let released = false;
        const release = () => {
            if (released) return;
            released = true;
            setPhase("opening");
            setTransform("none");
        };

        const frame = requestAnimationFrame(() => requestAnimationFrame(release));
        // A browser suspends requestAnimationFrame in a tab that is not being drawn - a
        // background tab, another window in front. Without this the panel would sit at the size
        // of the thing it came from, for as long as the tab stayed hidden, and be unusable when
        // it came back. The animation is worth having; the panel opening at all is worth more.
        const fallback = setTimeout(release, 100);

        return () => {
            cancelAnimationFrame(frame);
            clearTimeout(fallback);
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const close = useCallback(() => {
        if (closingRef.current) return;
        closingRef.current = true;

        // Nothing to shrink into - opened without an origin, so there is no journey to reverse.
        if (!rect) return onClose?.();

        setPhase("closing");
        setTransform(transformTo(rect, panelRef.current?.getBoundingClientRect()));

        // transitionend rather than a timer, so no duration is repeated in two places to drift
        // apart. The fallback covers the case where no transition runs at all - reduced motion,
        // or a backgrounded tab - where transitionend never fires and the panel would hang on
        // screen for ever.
        let done = false;
        const finish = () => {
            if (done) return;
            done = true;
            onClose?.();
        };
        panelRef.current?.addEventListener(
            "transitionend",
            (e) => e.propertyName === "transform" && finish(),
            { once: true }
        );
        setTimeout(finish, SAFETY_MS);
    }, [rect, onClose]);

    useEffect(() => {
        const onKey = (e) => e.key === "Escape" && close();
        document.addEventListener("keydown", onKey);
        return () => document.removeEventListener("keydown", onKey);
    }, [close]);

    return { panelRef, transform, phase, close };
}
