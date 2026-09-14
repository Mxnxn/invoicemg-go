export const GETALLCLIENT = "GETALLCLIENT";
export const GETUSER = "GETUSER";
export const GETDATES = "GETDATES";
export const GETINVOICES = "GETDATES";

export const SAVE = "SAVE";

export const getAllClients = (state) => {
    return { type: GETALLCLIENT, state: state };
};
export const getAllDates = (state) => {
    return { type: GETDATES, state: state };
};
export const getAdminDetails = (state) => {
    return { type: GETUSER, state: state };
};
export const getAllGeneratedInvoices = (state) => {
    return { type: GETINVOICES, state: state };
};
