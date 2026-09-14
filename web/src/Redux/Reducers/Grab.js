import { GETALLCLIENT, GETUSER, GETDATES, GETINVOICES } from "../Actions/Grab";

const initial_state = {
    clients: [],
    dates: [],
    invoices: [],
    user: {},
};

const grabReducer = (state = initial_state, action) => {
    switch (action.type) {
        case GETALLCLIENT:
            state.clients = action.state;
            return state;
        case GETDATES:
            state.dates = action.state;
            return state;
        case GETINVOICES:
            state.invoices = action.state;
            return state;
        case GETUSER:
            state.user = action.state;
            return state;
        default:
            return state;
    }
};

export default grabReducer;
