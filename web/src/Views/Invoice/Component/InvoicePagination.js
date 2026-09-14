import React from "react";
import Pagination from "../../../Shell/Pagination";

const InvoicePagination = ({ invoices, invoicesPerPage, setCurrentPage, currentPage }) => (
    <Pagination totalItems={invoices.length} perPage={invoicesPerPage} currentPage={currentPage} setCurrentPage={setCurrentPage} />
);

export default InvoicePagination;
