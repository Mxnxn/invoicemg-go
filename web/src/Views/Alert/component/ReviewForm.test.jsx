import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const reviewStatus = vi.fn();
const submitReview = vi.fn();
vi.mock("../alert_backend", () => ({
    alertBackend: {
        reviewStatus: (...a) => reviewStatus(...a),
        submitReview: (...a) => submitReview(...a),
    },
}));

import ReviewForm from "./ReviewForm";

const scores = { quality: 5, speed: 4, communication: 3, satisfaction: 4, overall: 4 };

beforeEach(() => {
    vi.clearAllMocks();
    reviewStatus.mockResolvedValue({ data: { reviewed: false, review: null } });
    submitReview.mockResolvedValue({ data: { scores, comment: "" } });
});

const rate = (label, star) => {
    const group = screen.getByLabelText(label).closest(".review-row").querySelector(".review-stars");
    fireEvent.click(group.querySelectorAll("button")[star - 1]);
};

describe("ReviewForm", () => {
    it("asks for all five dimensions", async () => {
        render(<ReviewForm jobId="j1" jobcardId="c1" />);
        await waitFor(() => expect(screen.getByText("How did we do?")).toBeDefined());
        for (const label of ["Quality of work", "Speed", "Communication", "Satisfaction", "Overall"]) {
            expect(screen.getByText(label)).toBeDefined();
        }
    });

    // A partial review is an unfinished form, not a partial opinion - sending four scores
    // would drag the fifth dimension's average down by a rating nobody gave.
    it("refuses to send until all five are rated", async () => {
        render(<ReviewForm jobId="j1" jobcardId="c1" />);
        await waitFor(() => expect(screen.getByText("Send review")).toBeDefined());
        expect(screen.getByText("Send review").disabled).toBe(true);

        rate("Quality of work", 5);
        rate("Speed", 4);
        rate("Communication", 3);
        rate("Satisfaction", 4);
        expect(screen.getByText("Send review").disabled).toBe(true);

        rate("Overall", 4);
        expect(screen.getByText("Send review").disabled).toBe(false);
    });

    it("sends every score, and the comment when given", async () => {
        render(<ReviewForm jobId="j1" jobcardId="c1" />);
        await waitFor(() => expect(screen.getByText("Send review")).toBeDefined());
        rate("Quality of work", 5);
        rate("Speed", 4);
        rate("Communication", 3);
        rate("Satisfaction", 4);
        rate("Overall", 4);
        fireEvent.change(screen.getByPlaceholderText(/What went well/), { target: { value: "Great job" } });
        fireEvent.click(screen.getByText("Send review"));

        await waitFor(() => expect(submitReview).toHaveBeenCalled());
        const [jobId, cardId, form] = submitReview.mock.calls[0];
        expect(jobId).toBe("j1");
        expect(cardId).toBe("c1");
        expect(form.get("quality")).toBe("5");
        expect(form.get("overall")).toBe("4");
        expect(form.get("comment")).toBe("Great job");
    });

    // Already reviewed: show what they said. Offering the form again invites a duplicate the
    // server will refuse, which reads as the review not having been saved.
    it("shows the existing review instead of the form", async () => {
        reviewStatus.mockResolvedValue({ data: { reviewed: true, review: { scores, comment: "Lovely" } } });
        render(<ReviewForm jobId="j1" jobcardId="c1" />);
        await waitFor(() => expect(screen.getByText("Your review")).toBeDefined());
        expect(screen.queryByText("Send review")).toBeNull();
        expect(screen.getByText(/Lovely/)).toBeDefined();
    });

    // This page is outside the admin shell, so the global toast interceptor is not what a
    // customer sees - the failure has to land on the form itself.
    it("shows the failure on the form, since no toast reaches this page", async () => {
        submitReview.mockRejectedValue({ message: "You have already reviewed this job. Thank you!" });
        render(<ReviewForm jobId="j1" jobcardId="c1" />);
        await waitFor(() => expect(screen.getByText("Send review")).toBeDefined());
        ["Quality of work", "Speed", "Communication", "Satisfaction", "Overall"].forEach((l) => rate(l, 4));
        fireEvent.click(screen.getByText("Send review"));
        await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
        expect(screen.getByRole("alert").textContent).toMatch(/already reviewed/);
    });
});
