import axios from "axios";

class AnalyticsBackend {
    cashflow(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/cashflow`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // Customer reviews, aggregated - averages, the count behind them, the spread of the
    // overall score, and the most recent few with who left them.
    getReviews(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/reviews`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getProductionWip(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/production/wip`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getProductionThroughput(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/production/throughput`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getRevenue(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/revenue`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getTopSales(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/top-sales`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getTopCredits(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/top-credits`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getTopPaid(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/top-paid`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getAvgPaymentTime(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/avg-payment-time`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getAvgPendingTime(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/avg-pending-time`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getPayoutWeekday(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/payout-weekday`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getPayables(formData, stoken) {
        return new Promise((resolve, reject) => {
            axios
                .post(`${import.meta.env.VITE_API_URL}/analytics/payables`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                })
                .then((res) => {
                    resolve(res.data);
                })
                .catch((error) => {
                    reject(error);
                });
        });
    }

    getAging(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/aging`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getUnbilled(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/analytics/unbilled`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export let analyticsBackend = new AnalyticsBackend();
