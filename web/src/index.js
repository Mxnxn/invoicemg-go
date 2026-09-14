import React from "react";
import { applyAppearance, readAppearance } from "./Common/appearance";
import { createRoot } from "react-dom/client";

import "./assets/scss/argon-dashboard-react.scss";
import "./styles/tokens.css";
import "./styles/uiFonts.css";

// Applied before first paint (not inside a component) so the whole app - including
// pre-auth pages that render before AdminLayout ever mounts - starts in the right
// theme instead of flashing light and then flipping dark.
document.documentElement.setAttribute("data-theme", window.localStorage.getItem("theme") || "light");
// Same reason as the theme above: applied before the first paint so text does not render at
// the default size and then jump to the chosen one.
applyAppearance(readAppearance());
import { Provider } from "react-redux";
import App from "./App/App";
import grabReducer from "./Redux/Reducers/Grab";
import { createStore, combineReducers } from "redux";
import "./api/errorInterceptor";
const rootReducer = combineReducers({
    grabReducer: grabReducer,
});

function loadFromLocal() {
    try {
        const state = localStorage.getItem("redux");
        if (state === null) return undefined;
        return JSON.parse(state);
    } catch (err) {
        console.log(err);
    }
}

const persistedState = loadFromLocal();

const store = createStore(rootReducer, persistedState);

store.subscribe(() => {
    saveToLocal(store.getState());
});

function saveToLocal(statex) {
    try {
        const state = JSON.stringify(statex);
        localStorage.setItem("redux", state);
    } catch (err) {
        console.log(err);
    }
}
createRoot(document.getElementById("root")).render(
    <Provider store={store}>
        <App />
    </Provider>
);
