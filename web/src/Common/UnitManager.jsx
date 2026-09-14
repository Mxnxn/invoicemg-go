import React, { useCallback, useEffect, useState } from "react";
import { Edit, Plus, Trash2 } from "react-feather";

import { unitBackend } from "./unit_backend";
import DataTable from "./DataTable/DataTable";
import RowActionMenu, { RowActionMenuItem } from "./DataTable/RowActionMenu";
import ConfirmDialog from "./ConfirmDialog";
import UnitFormModal from "./UnitFormModal";
import { useUndoDelete } from "./undoDelete";

// The units a product can be stocked in, below the products table in Configure > Products.
// Same shape as every other manager now: a table with a Create button, one dialog for create
// and edit, and the undo bar for deletes.
const UnitManager = () => {
    const [units, setUnits] = useState([]);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const { scheduleDelete } = useUndoDelete();

    // null = creating, a unit = editing. One dialog serves both.
    const [formOpen, setFormOpen] = useState(false);
    const [editing, setEditing] = useState(null);

    const getUnits = useCallback(async () => {
        try {
            const res = await unitBackend.list();
            setUnits(res.data);
        } catch (error) {
            console.log(error);
        }
    }, []);

    useEffect(() => {
        getUnits();
    }, [getUnits]);

    const onCreateClick = () => {
        setEditing(null);
        setFormOpen(true);
    };

    const onEditClick = (unit) => {
        setEditing(unit);
        setFormOpen(true);
        setMoreMenu(-1);
    };

    const confirmDelete = () => {
        if (!deleteTarget) return;
        const id = deleteTarget;
        const removed = units.find((u) => u._id === id);
        const index = units.findIndex((u) => u._id === id);
        setUnits((prev) => prev.filter((u) => u._id !== id));
        setDeleteTarget(null);
        scheduleDelete({
            label: "unit",
            commit: () => {
                const formData = new FormData();
                formData.set("unit_id", id);
                return unitBackend.delete(formData);
            },
            undo: () => setUnits((prev) => [...prev.slice(0, index), removed, ...prev.slice(index)]),
        });
    };

    return (
        <>
            <div className="shell-card mt-3">
                <div className="shell-card-header">
                    <span className="text-heading-brand">Units</span>
                    <button type="button" className="shell-btn shell-btn-sm shell-btn-primary" onClick={onCreateClick}>
                        <Plus size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
                        Create Unit
                    </button>
                </div>
                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">Name</th>
                            <th scope="col">Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {units.length === 0 ? (
                            <tr>
                                <td colSpan={2} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                    No units yet.
                                </td>
                            </tr>
                        ) : (
                            units.map((unit, index) => (
                                <tr key={unit._id}>
                                    <td>{unit.name}</td>
                                    <td>
                                        <RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
                                            <RowActionMenuItem icon={Edit} variant="warning" onClick={() => onEditClick(unit)}>
                                                Edit
                                            </RowActionMenuItem>
                                            <RowActionMenuItem
                                                icon={Trash2}
                                                variant="danger"
                                                onClick={() => {
                                                    setDeleteTarget(unit._id);
                                                    setMoreMenu(-1);
                                                }}
                                            >
                                                Delete
                                            </RowActionMenuItem>
                                        </RowActionMenu>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </DataTable>
            </div>

            <UnitFormModal
                isOpen={formOpen}
                toggle={() => setFormOpen(false)}
                unit={editing}
                onCreated={(created) => setUnits((prev) => [...prev, created])}
                onUpdated={(saved) => setUnits((prev) => prev.map((u) => (u._id === saved._id ? saved : u)))}
            />

            <ConfirmDialog
                open={!!deleteTarget}
                message="Delete this unit? Purchase invoice rows already using it keep their saved value."
                confirmLabel="Delete"
                position="bottom"
                danger
                onConfirm={confirmDelete}
                onCancel={() => setDeleteTarget(null)}
            />
        </>
    );
};

export default UnitManager;
