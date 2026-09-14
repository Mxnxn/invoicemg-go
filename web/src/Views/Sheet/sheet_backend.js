import axios from "axios";
class SheetsBackend {
	getAllSheets(formData, stoken) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/sheet/only`,
					formData,
					{
						headers: {
							"SESSION-TOKEN": stoken,
						},
					}
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	getSheetDetail(formData, stoken) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/sheet/get`,
					formData,
					{
						headers: {
							"SESSION-TOKEN": stoken,
						},
					}
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	// Job-ids with at least one card short of Done. Surfaced on the dashboard so work still
	// open is visible beside the days it came in on.
	getOpenJobs() {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/sheet/open-jobs`,
					new FormData(),
					{ headers: { "SESSION-TOKEN": window.localStorage.getItem("session_token") } }
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
}

export let sheetsBackend = new SheetsBackend();
