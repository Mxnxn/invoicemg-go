import { useEffect, useMemo, useState } from "react";
import { readAppearance } from "./appearance";
import { APPEARANCE_EVENT } from "./appearanceSync";

// Paging for a table, driven by the Rows-per-page setting in Account settings > Appearance.
//
// Every table that wants paging calls this instead of keeping its own page size, so the one
// setting reaches all of them and none of them can disagree about what "25" means.
//
//   const { pageRows, page, setPage, perPage, total } = usePagedRows(rows);
//   {pageRows.map(...)}
//   <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
//
// perPage 0 is "All": the slice is skipped entirely and Pagination renders nothing, which is
// exactly how these tables behaved before the setting existed.
export function useRowsPerPage() {
    const [rowsPerPage, setRowsPerPage] = useState(() => readAppearance().rowsPerPage);

    // Settings live in another view, so a change has to reach mounted tables somehow. The
    // event fires on save; `storage` covers the same person changing it in another tab.
    useEffect(() => {
        const sync = () => setRowsPerPage(readAppearance().rowsPerPage);
        window.addEventListener(APPEARANCE_EVENT, sync);
        window.addEventListener("storage", sync);
        return () => {
            window.removeEventListener(APPEARANCE_EVENT, sync);
            window.removeEventListener("storage", sync);
        };
    }, []);

    return rowsPerPage;
}

// `fixedPerPage` opts a view out of the setting. Rows-per-page is about TABLE rows; a grid of
// cards is laid out by column width, so the number that fills it tidily is a property of the
// layout rather than a reading preference. Pass nothing and the setting still wins, which is
// what every table does.
// Nullish-coalesced, not ||, because 0 is a meaningful override: it means "All".
export function usePagedRows(rows, fixedPerPage) {
    const setting = useRowsPerPage();
    const perPage = fixedPerPage ?? setting;
    const [page, setPage] = useState(1);
    const list = useMemo(() => rows || [], [rows]);

    // Deleting the last row of the last page, or filtering the list down, would otherwise
    // leave the table showing an empty page with no way back except the pager.
    const pageCount = perPage > 0 ? Math.max(1, Math.ceil(list.length / perPage)) : 1;
    useEffect(() => {
        if (page > pageCount) setPage(pageCount);
    }, [page, pageCount]);

    const pageRows = useMemo(() => {
        if (!perPage) return list;
        const start = (Math.min(page, pageCount) - 1) * perPage;
        return list.slice(start, start + perPage);
    }, [list, page, pageCount, perPage]);

    return { pageRows, page, setPage, perPage, total: list.length, pageCount };
}

export default usePagedRows;
