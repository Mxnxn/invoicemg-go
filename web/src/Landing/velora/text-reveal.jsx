// Ported from velora-ui-main/src/components/velora/text-reveal.tsx
import { Fragment } from "react";
import { motion, useReducedMotion } from "motion/react";

import { cn } from "../lib/cn";

export function TextReveal({ text, className, as: Tag = "span", delay = 0, stagger = 0.045, once = true }) {
    const reducedMotion = useReducedMotion();
    const words = text.split(" ");

    if (reducedMotion) {
        return <Tag className={cn(className)}>{text}</Tag>;
    }

    return (
        <Tag data-slot="text-reveal" className={cn(className)}>
            <span className="tw:sr-only">{text}</span>
            <motion.span
                aria-hidden
                initial="hidden"
                whileInView="visible"
                viewport={{ once, margin: "0px 0px -10% 0px" }}
                transition={{ staggerChildren: stagger, delayChildren: delay }}
                className="tw:inline"
            >
                {words.map((word, i) => (
                    // The separating space is a sibling of the word, not part of
                    // it: a trailing space inside an inline-block is trimmed, which
                    // runs the whole headline together.
                    <Fragment key={`${word}-${i}`}>
                        <motion.span
                            className="tw:inline-block tw:will-change-transform"
                            variants={{
                                hidden: { opacity: 0, y: 12, filter: "blur(8px)" },
                                visible: {
                                    opacity: 1,
                                    y: 0,
                                    filter: "blur(0px)",
                                    transition: { duration: 0.45, ease: [0.21, 0.47, 0.32, 0.98] },
                                },
                            }}
                        >
                            {word}
                        </motion.span>
                        {i < words.length - 1 && " "}
                    </Fragment>
                ))}
            </motion.span>
        </Tag>
    );
}
