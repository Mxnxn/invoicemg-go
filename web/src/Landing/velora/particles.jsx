// Ported from velora-ui-main/src/components/velora/particles.tsx
import { useEffect, useRef } from "react";

import { cn } from "../lib/cn";

/**
 * Drifting, twinkling particle field on a lightweight canvas. Renders a single
 * static frame when reduced motion is preferred.
 */
export function Particles({ className, quantity = 60, color }) {
    const canvasRef = useRef(null);

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext("2d");
        if (!ctx) return;

        const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        const dpr = Math.min(window.devicePixelRatio || 1, 2);
        let particles = [];
        let raf = 0;
        let width = 0;
        let height = 0;

        // The canvas inherits `color` from its context, so particles match the
        // current theme. Computed colors resolve to rgb() in browsers.
        const resolveColor = () => {
            if (color) return color;
            const match = getComputedStyle(canvas).color.match(/\d+(\.\d+)?/g);
            return match ? match.slice(0, 3).join(", ") : "255, 255, 255";
        };
        let rgb = "255, 255, 255";

        const resize = () => {
            const rect = canvas.getBoundingClientRect();
            width = rect.width;
            height = rect.height;
            canvas.width = width * dpr;
            canvas.height = height * dpr;
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            rgb = resolveColor();
            particles = Array.from({ length: quantity }, () => ({
                x: Math.random() * width,
                y: Math.random() * height,
                vx: (Math.random() - 0.5) * 0.15,
                vy: (Math.random() - 0.5) * 0.15,
                size: Math.random() * 1.6 + 0.4,
                alpha: Math.random() * 0.5 + 0.1,
                twinkle: Math.random() * 0.02 + 0.005,
            }));
        };

        const draw = () => {
            ctx.clearRect(0, 0, width, height);
            for (const p of particles) {
                ctx.beginPath();
                ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
                ctx.fillStyle = `rgba(${rgb}, ${p.alpha})`;
                ctx.fill();
            }
        };

        const tick = () => {
            for (const p of particles) {
                p.x = (p.x + p.vx + width) % width;
                p.y = (p.y + p.vy + height) % height;
                p.alpha += p.twinkle;
                if (p.alpha > 0.6 || p.alpha < 0.1) p.twinkle *= -1;
            }
            draw();
            raf = requestAnimationFrame(tick);
        };

        resize();
        if (reduced) {
            draw();
        } else {
            raf = requestAnimationFrame(tick);
        }

        // jsdom has neither observer, so guard both for the test environment.
        const resizeObserver = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(resize);
        resizeObserver?.observe(canvas);

        // Re-resolve the colour when the theme flips. The admin panel switches
        // themes via html[data-theme], not a class.
        const themeObserver = typeof MutationObserver === "undefined" ? null : new MutationObserver(() => {
            rgb = resolveColor();
            if (reduced) draw();
        });
        themeObserver?.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ["data-theme"],
        });

        return () => {
            cancelAnimationFrame(raf);
            resizeObserver?.disconnect();
            themeObserver?.disconnect();
        };
    }, [quantity, color]);

    return (
        <canvas
            ref={canvasRef}
            aria-hidden
            data-slot="particles"
            className={cn("tw:pointer-events-none tw:absolute tw:inset-0 tw:size-full", className)}
        />
    );
}
