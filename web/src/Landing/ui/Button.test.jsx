import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";

describe("Button", () => {
    it("renders its children in a button element", () => {
        render(<Button>Sign in</Button>);
        expect(screen.getByRole("button", { name: "Sign in" })).toBeDefined();
    });

    it("applies only prefixed tailwind classes", () => {
        render(<Button>Sign in</Button>);
        const classes = screen.getByRole("button").className.split(/\s+/).filter(Boolean);
        expect(classes.every((c) => c.startsWith("tw:"))).toBe(true);
    });

    it("merges a caller-supplied className", () => {
        render(<Button className="tw:w-full">Sign in</Button>);
        expect(screen.getByRole("button").className).toContain("tw:w-full");
    });

    it("forwards arbitrary props such as onClick and type", () => {
        render(<Button type="submit">Go</Button>);
        expect(screen.getByRole("button").getAttribute("type")).toBe("submit");
    });
});
