import React from "react";
import { Save, RotateCcw } from "react-feather";

// The save control for the settings tabs in Configure > Templates.
//
// Numbering saved per row, with four separate buttons; Export configs saved all six of its
// toggles from one button at the bottom. Neither told you anything was unsaved, so the two
// tabs disagreed about what a change even was - and on Numbering it was possible to edit
// three rows and save one. One bar, shared, appears only once something has actually
// changed, and says what will happen to it.
//
// `dirty` drives everything: when nothing has changed there is no bar, so the resting state
// of the page is not a row of buttons that do nothing.
const SaveBar = ({ dirty, saving, onSave, onDiscard, disabled, note }) => {
    if (!dirty) {
        return note ? (
            <p className="save-bar-note text-body-small" role="status">
                {note}
            </p>
        ) : null;
    }

    return (
        <div className="save-bar" role="status">
            <span className="save-bar-label text-body-small">Unsaved changes</span>
            <button type="button" className="shell-btn shell-btn-sm shell-btn-secondary" onClick={onDiscard} disabled={saving}>
                <RotateCcw size={13} aria-hidden="true" />
                Discard
            </button>
            <button type="button" className="shell-btn shell-btn-sm shell-btn-primary" onClick={onSave} disabled={saving || disabled}>
                <Save size={13} aria-hidden="true" />
                {saving ? "Saving…" : "Save changes"}
            </button>
        </div>
    );
};

export default SaveBar;
