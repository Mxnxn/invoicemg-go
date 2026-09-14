import React, { useEffect, useState } from "react";
import { Star, CheckCircle } from "react-feather";
import { alertBackend } from "../alert_backend";
import "./reviewForm.css";

// The customer's review of the job, on the page they reach from the WhatsApp link.
//
// Five scores out of 5. `overall` is asked rather than averaged from the other four: someone
// can rate the work 5 and the communication 2 and still call the job a 4, and computing it
// would put words in their mouth.
//
// No login, because the person rating the work is a customer and there is never going to be
// one. What authorises it is the same pair of ids that authorised reading the page.
//
// One review per job. The page asks on load and shows what they said rather than an empty
// form, because a customer who taps the link twice should not be invited to review twice.
const DIMENSIONS = [
    { key: "quality", label: "Quality of work" },
    { key: "speed", label: "Speed" },
    { key: "communication", label: "Communication" },
    { key: "satisfaction", label: "Satisfaction" },
    { key: "overall", label: "Overall" },
];

const STARS = [1, 2, 3, 4, 5];

// A radio group, not five buttons: it is one choice out of five, so arrow keys should move
// between them and a screen reader should announce it as a single question.
const StarRow = ({ name, label, value, onChange, readOnly }) => (
    <div className="review-row">
        <span className="review-row-label" id={`${name}-label`}>
            {label}
        </span>
        <div
            className="review-stars"
            role={readOnly ? "img" : "radiogroup"}
            aria-labelledby={`${name}-label`}
            aria-label={readOnly ? `${label}: ${value} out of 5` : undefined}
        >
            {STARS.map((star) => {
                const filled = star <= value;
                if (readOnly) {
                    return (
                        <span key={star} className={`review-star${filled ? " is-filled" : ""}`} aria-hidden="true">
                            <Star size={22} />
                        </span>
                    );
                }
                return (
                    <button
                        key={star}
                        type="button"
                        role="radio"
                        aria-checked={value === star}
                        aria-label={`${star} out of 5`}
                        className={`review-star${filled ? " is-filled" : ""}`}
                        onClick={() => onChange(star)}
                    >
                        <Star size={22} />
                    </button>
                );
            })}
        </div>
    </div>
);

const ReviewForm = ({ jobId, jobcardId }) => {
    const [scores, setScores] = useState({});
    const [comment, setComment] = useState("");
    const [existing, setExisting] = useState(null);
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");
    const [justSent, setJustSent] = useState(false);

    useEffect(() => {
        let live = true;
        alertBackend
            .reviewStatus(jobId, jobcardId)
            .then((res) => {
                if (live && res?.data?.reviewed) setExisting(res.data.review);
            })
            // A failed check must not hide the form - the worst case is the server refuses a
            // duplicate, which it does anyway and says so.
            .catch(() => {})
            .finally(() => {
                if (live) setLoading(false);
            });
        return () => {
            live = false;
        };
    }, [jobId, jobcardId]);

    const complete = DIMENSIONS.every((d) => scores[d.key] >= 1);

    const submit = async () => {
        if (!complete || busy) return;
        setBusy(true);
        setError("");
        try {
            const formData = new FormData();
            DIMENSIONS.forEach((d) => formData.set(d.key, String(scores[d.key])));
            if (comment.trim()) formData.set("comment", comment.trim());
            const res = await alertBackend.submitReview(jobId, jobcardId, formData);
            setExisting({ scores: res.data.scores, comment: res.data.comment });
            setJustSent(true);
        } catch (err) {
            // This page is outside the admin shell, so the global toast interceptor is not
            // what a customer sees - the message has to land on the form itself.
            setError(err?.message || "Could not send your review. Please try again.");
        } finally {
            setBusy(false);
        }
    };

    if (loading) return null;

    if (existing) {
        return (
            <section className="review-card" aria-labelledby="review-heading">
                <h2 className="review-heading" id="review-heading">
                    {justSent ? (
                        <>
                            <CheckCircle size={17} aria-hidden="true" /> Thank you
                        </>
                    ) : (
                        "Your review"
                    )}
                </h2>
                <p className="review-intro">
                    {justSent
                        ? "Your feedback has been sent to the team."
                        : "You have already reviewed this job."}
                </p>
                {DIMENSIONS.map((d) => (
                    <StarRow
                        key={d.key}
                        name={`ro-${d.key}`}
                        label={d.label}
                        value={existing.scores?.[d.key] || 0}
                        readOnly
                    />
                ))}
                {existing.comment ? <p className="review-quote">“{existing.comment}”</p> : null}
            </section>
        );
    }

    return (
        <section className="review-card" aria-labelledby="review-heading">
            <h2 className="review-heading" id="review-heading">
                How did we do?
            </h2>
            <p className="review-intro">
                Rate this job out of 5. It takes a moment and it goes straight to the team.
            </p>

            {DIMENSIONS.map((d) => (
                <StarRow
                    key={d.key}
                    name={d.key}
                    label={d.label}
                    value={scores[d.key] || 0}
                    onChange={(star) => setScores((prev) => ({ ...prev, [d.key]: star }))}
                />
            ))}

            <label className="review-comment-label" htmlFor="review-comment">
                Anything else? <span className="review-optional">(optional)</span>
            </label>
            <textarea
                id="review-comment"
                className="review-comment"
                rows={3}
                maxLength={1000}
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                placeholder="What went well, or what could be better?"
            />

            {error ? (
                <p className="review-error" role="alert">
                    {error}
                </p>
            ) : null}

            <button type="button" className="review-submit" onClick={submit} disabled={!complete || busy}>
                {busy ? "Sending…" : "Send review"}
            </button>
            {/* Stated rather than left to be discovered by a disabled button that never
                explains itself. */}
            {!complete && <p className="review-hint">Please rate all five to send.</p>}
        </section>
    );
};

export default ReviewForm;
