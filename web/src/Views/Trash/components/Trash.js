import React, { useEffect, useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";
import { trashBackend } from "../trash_backend";
import { Container, Row, Col } from "reactstrap";
import LiteHeader from "../../../Common/Header/LiteHeader";
import SetPassword from "./SetPassword";
import AskPassword from "./AskPassword";
import { getDate } from "../../../Common/DateAndTime/getDate";
import DataTable from "../../../Common/DataTable/DataTable";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import RowCheckbox from "../../../Common/DataTable/RowCheckbox";
import useRowSelection from "../../../Common/DataTable/useRowSelection";

const Trash = (props) => {
	const [modal, setModal] = useState({
		setPassword: false,
		askPassword: false,
	});

	const [password, setPassword] = useState({ value: "", error: false });
	// `ready` - the data has arrived and the page may render. NOT a loading flag, despite what
	// it used to be called: it starts false and is set true AFTER the fetch, and the whole page
	// is gated on it below.
	//
	// Renamed because the name caused a real bug. Wiring the shimmer in read it as "is
	// loading" and passed it straight to DataTable, which inverted it - the table then
	// shimmered for ever, starting the moment the rows were actually available.
	const [ready, setReady] = useState(false);
	const [entries, setEntries] = useState([]);

	const entrySelection = useRowSelection(entries);

	// The vault has three states, and the server now names them in `state`. This used to
	// compare res.message against exact English sentences, so any copy change silently left
	// the page with no modal open and nothing rendered.
	const getTrash = useCallback(async () => {
		try {
			const formData = new FormData();
			formData.set("uid", window.localStorage.getItem("uid"));
			const res = await trashBackend.getAllEntry(formData);
			const state = res.state || (res.message === "Set a password first." ? "set-password" : res.message === "Password Require." ? "password-required" : "unlocked");
			if (state === "set-password") {
				return setModal({ setPassword: true, askPassword: false });
			}
			if (state === "password-required") {
				return setModal({ setPassword: false, askPassword: true });
			}
			setEntries(res.data || []);
			setReady(true);
		} catch (error) {
			// A failure here left a blank screen with the reason only in the console. The
			// interceptor has already toasted it; put the unlock prompt up so there's a way out.
			setModal({ setPassword: false, askPassword: true });
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const onSubmitPassword = async () => {
		// Was a missing return - it set the error and then submitted the empty password anyway.
		if (!password.value) {
			return setPassword({ ...password, error: "It can't be empty." });
		}
		try {
			const formData = new FormData();
			formData.set("uid", window.localStorage.getItem("uid"));
			formData.set("password", password.value);
			const res = await trashBackend.getAllEntry(formData);
			setEntries(res.data || []);
			setReady(true);
			setPassword({ value: "", error: false });
			closeModals();
		} catch (error) {
			// Any rejection here means the unlock didn't happen; say so on the field rather
			// than only for one specific message.
			return setPassword({ ...password, error: error.message || "Couldn't unlock the vault." });
		}
	};

	const navigate = useNavigate();

	// Just dismiss the dialogs and stay put. This is what a SUCCESSFUL unlock wants - the
	// user is here to see the vault, so navigating away would undo what they just did.
	const closeModals = () => {
		setModal({
			setPassword: false,
			askPassword: false,
		});
	};

	// Cancelling is the opposite intent: the user declined to unlock, so leaving them parked
	// on a page they cannot see into is a dead end. Go back where they came from, falling
	// back to the dashboard when there is no in-app history (deep link, fresh tab).
	// react-router v6 tracks its own position in history.state.idx; idx 0 means this entry is
	// the first of the session, so navigate(-1) would leave the app entirely.
	const onCancelHandler = () => {
		closeModals();
		const idx = window.history.state && window.history.state.idx;
		if (typeof idx === "number" && idx > 0) {
			navigate(-1);
		} else {
			navigate("/admin/dashboard");
		}
	};

	useEffect(() => {
		getTrash();
	}, [getTrash]);

	return (
		<div>
			{/* Rendered straight away rather than held back until the data lands: the header and
			    the card do not depend on the rows, and the table shimmers in place while they load.
			    Gating the page on `ready` meant a blank screen instead of a page filling in. */}
			<LiteHeader bg="primary" />
				<Container className="" fluid>
					<Row>
						<Col className="mb-5 mb-xl-0" xl="12">
							<div className="shell-card">
								<div className="shell-card-header">
									<span className="text-heading-brand">Deleted Entries</span>
								</div>
								<DataTable loading={!ready}>
									<thead>
										<tr>
											<th scope="col" className="xan-col-checkbox">
												<RowCheckbox
													checked={entrySelection.allSelected}
													indeterminate={entrySelection.someSelected && !entrySelection.allSelected}
													onChange={entrySelection.toggleAll}
													ariaLabel="Select all entries"
												/>
											</th>
											<th scope="col">Date</th>
											<th scope="col">Customer</th>
											<th scope="col">Product</th>
											<th scope="col">Description</th>
											<th scope="col">Qty</th>
											<th scope="col">Rate</th>
											<th scope="col">SubTotal(&#8377;)</th>
											<th scope="col">Due(&#8377;)</th>
											<th scope="col">Received(&#8377;)</th>
										</tr>
									</thead>
									<tbody>
										{entries.map((entry, index) => (
											<tr
												key={entry._id || index}
												className={entrySelection.isSelected(entry._id) ? "selected" : ""}
											>
												<td className="xan-col-checkbox">
													<RowCheckbox
														checked={entrySelection.isSelected(entry._id)}
														onChange={() => entrySelection.toggle(entry._id)}
														ariaLabel={`Select entry ${index + 1}`}
													/>
												</td>
												<td className="cell-mono">{getDate(entry.date)}</td>
												<td>
													{entry.material}{" "}
													{entry.has_issued ? (
														<StatusBadge status="paid">issued</StatusBadge>
													) : (
														<StatusBadge status="pending">pending</StatusBadge>
													)}
												</td>
												<td>{entry.description}</td>
												<td className="cell-mono">{entry.qty}</td>
												<td className="cell-mono">{entry.rate}</td>
												<td className="cell-mono">&#8377;{entry.amount}</td>
												<td className="cell-mono">&#8377;{entry.total}</td>
												<td className="cell-mono">
													&#8377;{entry.advance}{" "}
													{entry.total === 0 ? (
														<StatusBadge status="paid">Paid</StatusBadge>
													) : (
														<StatusBadge status="pending">received</StatusBadge>
													)}
												</td>
											</tr>
										))}
									</tbody>
								</DataTable>
								<div className="shell-card-footer">
									<span className="text-body-small" style={{ color: "var(--status-amber-text)" }}>
										Info: Tap on Customer for entries
									</span>
								</div>
							</div>
						</Col>
					</Row>
				</Container>
			<SetPassword
				modal={modal.setPassword}
				onCancelHandler={onCancelHandler}
				// Re-check rather than reload the page: the vault is now set up, so this
				// lands straight on the unlock prompt.
				onPasswordSet={() => {
					setModal({ setPassword: false, askPassword: true });
				}}
			/>
			<AskPassword
				modal={modal.askPassword}
				onCancelHandler={onCancelHandler}
				error={password.error}
				Done={onSubmitPassword}
				password={password.value}
				setPassword={setPassword}
			/>
		</div>
	);
};

export default Trash;
