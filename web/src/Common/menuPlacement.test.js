import { describe, it, expect } from "vitest";
import { placeMenu, MENU_GAP } from "./menuPlacement";

// A phone: 390x844, and a picker sitting two thirds down the screen.
const trigger = { top: 560, bottom: 590, left: 16, width: 358 };

describe("placeMenu", () => {
    // Near the top of the screen, where the 260px menu genuinely fits underneath.
    it("sits below the trigger when there is room", () => {
        const high = { top: 200, bottom: 230, left: 16, width: 358 };
        const out = placeMenu({ trigger: high, viewportHeight: 844, menuHeight: 260 });

        expect(out.placement).toBe("below");
        expect(out.top).toBe(230 + MENU_GAP);
    });

    // The same trigger low on a full-height screen already lacks room below (844 - 590 = 254,
    // under the 260 the menu wants), so it flips even before a keyboard is involved - which is
    // why pickers near the bottom of a long form felt clipped on desktop too.
    it("flips for a low trigger even at full viewport height", () => {
        const out = placeMenu({ trigger, viewportHeight: 844, menuHeight: 260 });

        expect(out.placement).toBe("above");
    });

    // The bug this exists for. The on-screen keyboard leaves ~400px of visible viewport, so a
    // menu opening below a trigger at y=590 renders behind the keyboard: still open, still
    // focused, completely invisible. Flipping it above the trigger is what makes it usable.
    it("flips above the trigger when the keyboard has taken the space below", () => {
        const out = placeMenu({ trigger, viewportHeight: 400, menuHeight: 260 });

        expect(out.placement).toBe("above");
        expect(out.top).toBe(560 - 260 - MENU_GAP);
    });

    // Neither side fits: take the roomier one and shrink to it, rather than rendering a menu
    // that runs off an edge.
    it("shrinks to fit when neither side has room", () => {
        const out = placeMenu({ trigger: { top: 180, bottom: 210, left: 16, width: 358 }, viewportHeight: 320, menuHeight: 260 });

        expect(out.maxHeight).toBeLessThan(260);
        expect(out.maxHeight).toBeGreaterThan(0);
    });

    // Never above the top edge, whichever way it went.
    it("never places the menu off the top of the viewport", () => {
        const out = placeMenu({ trigger: { top: 40, bottom: 70, left: 16, width: 358 }, viewportHeight: 300, menuHeight: 260 });

        expect(out.top).toBeGreaterThanOrEqual(0);
    });

    // iOS reports the keyboard through visualViewport, which also carries an offset once the
    // page is scrolled under it - the menu has to be placed in that same coordinate space.
    it("accounts for a visual viewport offset", () => {
        const out = placeMenu({ trigger, viewportHeight: 400, viewportOffsetTop: 100, menuHeight: 260 });

        expect(out.placement).toBe("above");
        expect(out.top).toBe(560 - 260 - MENU_GAP);
    });

    it("keeps the trigger's horizontal position and width", () => {
        const out = placeMenu({ trigger, viewportHeight: 844, menuHeight: 260, width: 358 });

        expect(out.left).toBe(16);
        expect(out.width).toBe(358);
    });
});
