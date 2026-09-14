import { useEffect, useState } from "react";
import { Star } from "react-feather";
import ChartCard from "../charts/ChartCard";
import CategoryChart from "../charts/CategoryChart";
import { analyticsBackend } from "../analytics_backend";
import "./reviewsPanel.css";
import Blank from "../../../Common/DataTable/Blank";

// What customers said, from the review form on the public job page (Views/Alert).
//
// Two things this is careful to show, both because the average alone misleads:
//
//   THE COUNT travels with every average. "4.5" from two reviews and "4.5" from two hundred
//   are not the same claim, and a dashboard showing only the number invites the wrong one.
//
//   THE SPREAD of the overall score gets its own chart. An average of 3 built from two 5s and
//   two 1s describes a very different business from one built from four 3s, and no mean can
//   tell them apart.
//
// The five dimensions are shown as their own small rows rather than a chart: five bars whose
// values all sit between 3 and 5 is a chart that says less than the numbers do.
const DIMENSIONS = [
    { key: "quality", label: "Quality" },
    { key: "speed", label: "Speed" },
    { key: "communication", label: "Communication" },
    { key: "satisfaction", label: "Satisfaction" },
    { key: "overall", label: "Overall" },
];

// Filled to the rounded average, so 4.3 shows four. The number beside it carries the
// precision; the stars carry the shape.
const Stars = ({ value }) => (
    <span className="reviews-stars" aria-hidden="true">
        {[1, 2, 3, 4, 5].map((n) => (
            <Star key={n} size={13} className={n <= Math.round(value) ? "is-filled" : ""} />
        ))}
    </span>
);

export default function ReviewsPanel({ stoken }) {
    const [state, setState] = useState({ loading: true, error: "", data: null });

    useEffect(() => {
        let live = true;
        analyticsBackend
            .getReviews(new FormData(), stoken)
            .then((res) => {
                if (live) setState({ loading: false, error: "", data: res.data });
            })
            .catch(() => {
                if (live) setState({ loading: false, error: "Could not load reviews.", data: null });
            });
        return () => {
            live = false;
        };
    }, [stoken]);

    const data = state.data;
    const count = data?.count || 0;

    // Recharts wants an array; the API returns { 1: n, ... } keyed by score.
    const spread = [1, 2, 3, 4, 5].map((score) => ({
        name: `${score} star${score === 1 ? "" : "s"}`,
        value: data?.distribution?.[score] || 0,
    }));

    return (
        <ChartCard
            title="Customer reviews"
            subtitle={count === 1 ? "From 1 review" : `From ${count} reviews`}
            loading={state.loading}
            error={state.error}
            // No reviews yet is a normal state for a company that has just switched this on,
            // and ChartCard's empty copy says so better than a row of zeros would.
            empty={!state.loading && !state.error && count === 0}
            columns={[
                { key: "name", label: "Score" },
                { key: "value", label: "Reviews" },
            ]}
            rows={spread}
        >
            <div className="reviews-body">
                <div className="reviews-scores">
                    {DIMENSIONS.map((d) => {
                        const value = data?.averages?.[d.key] || 0;
                        return (
                            <div key={d.key} className={`reviews-row${d.key === "overall" ? " is-overall" : ""}`}>
                                <span className="reviews-label">{d.label}</span>
                                <Stars value={value} />
                                <span className="reviews-value">{value.toFixed(1)}</span>
                            </div>
                        );
                    })}
                </div>

                <div className="reviews-spread">
                    <span className="reviews-spread-title">Spread of overall score</span>
                    <CategoryChart data={spread} xKey="name" series={[{ key: "value", label: "Reviews" }]} />
                </div>
            </div>

            {/* Who said it. An average of 2.8 on communication is only actionable once you
                know which customers gave it. */}
            {data?.recent?.length > 0 && (
                <ul className="reviews-recent">
                    {data.recent.slice(0, 4).map((r) => (
                        <li key={r._id} className="reviews-recent-item">
                            <span className="reviews-recent-head">
                                <strong>{r.customer || <Blank />}</strong>
                                <span className="reviews-recent-job">{r.job}</span>
                                <Stars value={r.scores?.overall || 0} />
                            </span>
                            {r.comment ? <span className="reviews-recent-comment">“{r.comment}”</span> : null}
                        </li>
                    ))}
                </ul>
            )}
        </ChartCard>
    );
}
