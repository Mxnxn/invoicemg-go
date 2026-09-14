import { useMemo, useState } from "react";

// items: array of objects with an `id` field (or pass idKey to use a different field)
export default function useRowSelection(items, idKey = "_id") {
    const [selected, setSelected] = useState(() => new Set());

    const ids = useMemo(() => items.map((item) => item[idKey]), [items, idKey]);

    const isSelected = (id) => selected.has(id);

    const toggle = (id) => {
        setSelected((prev) => {
            const next = new Set(prev);
            next.has(id) ? next.delete(id) : next.add(id);
            return next;
        });
    };

    const allSelected = ids.length > 0 && ids.every((id) => selected.has(id));
    const someSelected = ids.some((id) => selected.has(id));

    const toggleAll = () => {
        setSelected((prev) => (allSelected ? new Set() : new Set(ids)));
    };

    const clear = () => setSelected(new Set());

    return {
        selected,
        selectedIds: Array.from(selected),
        isSelected,
        toggle,
        toggleAll,
        clear,
        allSelected,
        someSelected,
        count: selected.size,
    };
}
