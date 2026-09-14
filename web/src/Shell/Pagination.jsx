import React from "react";
import { ChevronLeft, ChevronRight } from "react-feather";
import "./shell.css";

const Pagination = ({ totalItems, perPage, currentPage, setCurrentPage }) => {
    const pageCount = Math.max(1, Math.ceil(totalItems / perPage));

    // A single page has nothing to navigate between - rendering the control just adds
    // chrome. Every table in the app funnels through here, so this covers all of them.
    if (pageCount < 2) return null;

    const pages = Array.from({ length: pageCount }, (_, i) => i + 1).filter(
        (p) => p === 1 || p === pageCount || Math.abs(p - currentPage) <= 1
    );

    return (
        <div className="shell-pagination">
            <button
                type="button"
                className="shell-page-btn"
                disabled={currentPage <= 1}
                onClick={() => setCurrentPage(currentPage - 1)}
                aria-label="Previous page"
            >
                <ChevronLeft size={15} />
            </button>
            {pages.map((page, i) => {
                const prev = pages[i - 1];
                const gap = prev && page - prev > 1;
                return (
                    <React.Fragment key={page}>
                        {gap && <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>…</span>}
                        <button
                            type="button"
                            className={["shell-page-btn text-body-regular", page === currentPage ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setCurrentPage(page)}
                        >
                            {page}
                        </button>
                    </React.Fragment>
                );
            })}
            <button
                type="button"
                className="shell-page-btn"
                disabled={currentPage >= pageCount}
                onClick={() => setCurrentPage(currentPage + 1)}
                aria-label="Next page"
            >
                <ChevronRight size={15} />
            </button>
        </div>
    );
};

export default Pagination;
