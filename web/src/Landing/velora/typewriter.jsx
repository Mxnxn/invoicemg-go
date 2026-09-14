// Ported from velora-ui-main/src/components/velora/typewriter.tsx
import { useEffect, useState } from "react";
import { useReducedMotion } from "motion/react";

import { cn } from "../lib/cn";

export function Typewriter({
    words,
    typeSpeed = 70,
    deleteSpeed = 40,
    holdTime = 1800,
    loop = true,
    cursor = true,
    className,
    ...props
}) {
    const reducedMotion = useReducedMotion();
    const [wordIndex, setWordIndex] = useState(0);
    const [text, setText] = useState("");
    const [deleting, setDeleting] = useState(false);

    useEffect(() => {
        if (reducedMotion || words.length === 0) return;
        const word = words[wordIndex % words.length];

        let timeout;
        if (!deleting && text.length < word.length) {
            timeout = setTimeout(() => setText(word.slice(0, text.length + 1)), typeSpeed);
        } else if (!deleting && text.length === word.length) {
            const isLast = wordIndex === words.length - 1;
            if (loop || !isLast) {
                timeout = setTimeout(() => setDeleting(true), holdTime);
            }
        } else if (deleting && text.length > 0) {
            timeout = setTimeout(() => setText(word.slice(0, text.length - 1)), deleteSpeed);
        } else if (deleting) {
            timeout = setTimeout(() => {
                setDeleting(false);
                setWordIndex((i) => (i + 1) % words.length);
            }, deleteSpeed);
        }
        return () => clearTimeout(timeout);
    }, [text, deleting, wordIndex, words, typeSpeed, deleteSpeed, holdTime, loop, reducedMotion]);

    if (reducedMotion) {
        return (
            <span className={cn(className)} {...props}>
                {words[0]}
            </span>
        );
    }

    const longest = words.reduce((a, b) => (b.length > a.length ? b : a), "");

    // Both spans share one grid cell: the invisible longest word reserves the
    // final size, so typing never shifts surrounding layout.
    return (
        <span data-slot="typewriter" className={cn("tw:inline-grid tw:text-left", className)} {...props}>
            <span className="tw:sr-only">{words[0]}</span>
            <span aria-hidden className="tw:invisible tw:col-start-1 tw:row-start-1">
                {longest}
                {cursor && <span className="tw:font-light">|</span>}
            </span>
            <span aria-hidden className="tw:col-start-1 tw:row-start-1">
                {text}
                {cursor && <span className="tw:animate-pulse tw:font-light">|</span>}
            </span>
        </span>
    );
}
