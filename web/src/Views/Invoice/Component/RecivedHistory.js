import React, { useState } from "react";
import { Trash2, X } from "react-feather";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import DataTable from "../../../Common/DataTable/DataTable";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";

// What a customer has paid, and when.
//
// A plain panel, NOT a portal. It used to wrap itself in SlideOverlay, which portals to
// document.body - fine on a full page, wrong inside a modal: it escaped the dialog it
// belonged to and covered the whole viewport instead of sitting beside the invoices. Its
// caller now places it, so the same component works as a column in a modal and as a column
// on the page.
//
// Typography comes from the .text-* scale rather than the legacy `geb fs-24`, so it matches
// the table beside it and responds to Appearance > Font size.
const ReceivedHistory = ({ historyArr = [], onClose, cname, onDeleteBtnHandler }) => {
    const [moreMenu, setMoreMenu] = useState(-1);

    return (
        <aside className="received-history" aria-label={`Payments from ${cname || "this customer"}`}>
            <div className="received-history-head">
                <div>
                    <p className="text-label-caps received-history-eyebrow">{cname ? `${cname}'s` : "Customer"}</p>
                    <h3 className="text-heading-brand received-history-title">Received</h3>
                </div>
                {onClose && (
                    <button type="button" className="slide-overlay-close" onClick={onClose} aria-label="Close payments">
                        <X size={16} />
                    </button>
                )}
            </div>

            {historyArr.length === 0 ? (
                <p className="text-body-small received-history-empty">Nothing received from this customer yet.</p>
            ) : (
                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">#</th>
                            <th scope="col">Date</th>
                            <th scope="col">Amount</th>
                            <th scope="col" aria-label="Actions" />
                        </tr>
                    </thead>
                    <tbody>
                        {historyArr.map((el, index) => (
                            <tr key={el._id || index}>
                                <td className="cell-mono">{index + 1}</td>
                                <td className="cell-mono">{getDate(el.createdAt)}</td>
                                <td className="cell-mono">₹{RoundOff(el.amount)}</td>
                                <td>
                                    <RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
                                        <RowActionMenuItem
                                            icon={Trash2}
                                            variant="danger"
                                            onClick={() => {
                                                onDeleteBtnHandler(el._id);
                                                setMoreMenu(-1);
                                            }}
                                        >
                                            Delete
                                        </RowActionMenuItem>
                                    </RowActionMenu>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}
        </aside>
    );
};

export default ReceivedHistory;
