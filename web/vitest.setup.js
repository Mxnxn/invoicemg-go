import { vi } from "vitest";

// Existing tests use the Jest globals (jest.fn, jest.mock, ...) carried over
// from the CRA/Jest setup — alias them onto Vitest's equivalent API.
globalThis.jest = vi;

// jsdom implements neither observer, but `motion` (landing page) constructs an
// IntersectionObserver for every whileInView/useInView element. Shim both, with
// the intersection observer reporting immediately so in-view content renders.
if (typeof globalThis.IntersectionObserver === "undefined") {
    globalThis.IntersectionObserver = class {
        constructor(callback) {
            this.callback = callback;
        }
        observe(target) {
            this.callback([{ target, isIntersecting: true, intersectionRatio: 1 }], this);
        }
        unobserve() {}
        disconnect() {}
        takeRecords() {
            return [];
        }
    };
}

if (typeof globalThis.ResizeObserver === "undefined") {
    globalThis.ResizeObserver = class {
        observe() {}
        unobserve() {}
        disconnect() {}
    };
}
